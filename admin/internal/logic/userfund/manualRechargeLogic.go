/*
 * 文件名称: manualRechargeLogic.go
 * 功能说明: 人工充值功能（管理员后台充值）
 *
 * 主要功能:
 * - 记录充值记录到 ae_user_recharge_record 表
 * - 更新用户日余额统计表 ae_user_balance_statistic_daily
 * - 使用 ON CONFLICT DO UPDATE 确保同一天的余额记录唯一性
 * - 返回充值后的新余额
 *
 * 数据操作:
 * 1. 插入充值记录:
 *    - charge_source = 1 (管理员后台充值)
 *    - xlcredit_amount = 充值金额
 *    - pay_time = 当前时间
 *
 * 2. 更新日余额统计:
 *    - 如果当天记录存在: balance = balance + 充值金额
 *    - 如果当天记录不存在: 创建新记录
 *
 * API路由: POST /manager/api/userfund/user/recharge
 * 请求体: {"user_id": "xxx", "amount": 100.00, "remarks": "活动赠送"}
 *
 * 安全提示:
 * - 前端有二次确认弹窗，避免误操作
 * - 充值成功后前端会显示Toast提示并刷新用户数据
 */

package userfund

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ManualRechargeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewManualRechargeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManualRechargeLogic {
	return &ManualRechargeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ManualRechargeLogic) ManualRecharge(req *types.ManualRechargeReq) (resp *types.ManualRechargeResp, err error) {
	operatorName := resolveOperatorName(l.ctx, l.svcCtx, req.UserId, 1)
	targetUsername := resolveTargetUsername(l.ctx, l.svcCtx, req.UserId)
	chargeType := req.ChargeType
	if chargeType <= 0 {
		chargeType = 1
	}

	// 1. 先获取用户当前最新余额（可能是今天或之前的记录）
	currentBalance, err := l.svcCtx.UserBalanceDailyModel.GetLatestBalance(l.ctx, req.UserId)
	if err != nil {
		l.Logger.Errorf("Failed to get current balance: %v", err)
		// 如果获取失败，默认使用 0
		currentBalance = 0
	}

	// 2. 解析可选过期时间（格式 YYYY-MM-DD，不传或空为永久有效）
	var expireTime sql.NullTime
	if strings.TrimSpace(req.ExpireTime) != "" {
		if t, parseErr := time.ParseInLocation("2006-01-02", strings.TrimSpace(req.ExpireTime), time.Local); parseErr == nil {
			// 设为该日 23:59:59，表示该日结束前有效
			endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
			expireTime = sql.NullTime{Time: endOfDay, Valid: true}
		}
	}

	// 3. 插入充值记录（含可选 expire_time）
	insertQuery := `
		INSERT INTO public.ae_user_recharge_record (
			user_id,
			xlcredit_amount,
			pay_time,
			create_time,
			update_time,
			charge_source,
			charge_type,
			remark,
			operator,
			expire_time
		)
		VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $8, $9)
		RETURNING id
	`
	fallbackInsertQuery := `
		INSERT INTO public.ae_user_recharge_record (
			user_id,
			xlcredit_amount,
			pay_time,
			create_time,
			update_time,
			charge_source,
			charge_type,
			remark,
			operator
		)
		VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $8)
		RETURNING id
	`
	now := time.Now()
	var insertedId int64
	err = l.svcCtx.DB.QueryRowCtx(l.ctx, &insertedId, insertQuery, req.UserId, req.Amount, now, now, now, chargeType, req.Remarks, operatorName, expireTime)
	if err != nil {
		l.Logger.Errorf("Failed to insert recharge record (full): %v, trying fallback", err)
		fallbackErr := l.svcCtx.DB.QueryRowCtx(l.ctx, &insertedId, fallbackInsertQuery, req.UserId, req.Amount, now, now, now, chargeType, req.Remarks, operatorName)
		if fallbackErr != nil {
			l.Logger.Errorf("Failed to insert recharge record (fallback): %v", fallbackErr)
			return &types.ManualRechargeResp{
				Success: false,
				Message: "充值记录插入失败",
			}, nil
		}
	}
	l.Logger.Infof("充值记录插入成功: record_id=%d user_id=%s amount=%.2f expire_time=%v", insertedId, req.UserId, req.Amount, expireTime)

	// 4. 更新日余额统计（使用 ON CONFLICT DO UPDATE）
	// 关键修复：如果当天没有记录，INSERT 时使用 当前余额 + 充值金额
	// 如果当天有记录，UPDATE 时使用 当天余额 + 充值金额
	updateQuery := `
		INSERT INTO ae_user_balance_statistic_daily (user_id, day, balance, create_time, update_time)
		VALUES ($1, CURRENT_DATE, $2, $3, $4)
		ON CONFLICT (user_id, day)
		DO UPDATE SET
			balance = ae_user_balance_statistic_daily.balance + $5,
			update_time = $4
		RETURNING balance
	`
	// 计算新余额：当前余额 + 充值金额（用于 INSERT，如果当天没有记录）
	newBalanceForInsert := currentBalance + req.Amount
	var newBalance float64
	err = l.svcCtx.DB.QueryRowCtx(l.ctx, &newBalance, updateQuery, req.UserId, newBalanceForInsert, now, now, req.Amount)
	if err != nil {
		l.Logger.Errorf("Failed to update balance: %v", err)
		return &types.ManualRechargeResp{
			Success: false,
			Message: "余额更新失败",
		}, nil
	}

	l.Logger.Infof("管理员资金操作: 操作管理员=%s, 操作用户=%s, user_id=%s, 类型=充值, 时间=%s, 原金额=%.2f, 变动金额=%.2f, 操作后金额=%.2f",
		operatorName, targetUsername, req.UserId, now.Format(time.RFC3339), currentBalance, req.Amount, newBalance)

	return &types.ManualRechargeResp{
		Success:    true,
		Message:    "充值成功",
		NewBalance: newBalance,
	}, nil
}
