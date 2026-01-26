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
 * 请求体: {"user_str_id": "xxx", "amount": 100.00, "remarks": "活动赠送"}
 *
 * 安全提示:
 * - 前端有二次确认弹窗，避免误操作
 * - 充值成功后前端会显示Toast提示并刷新用户数据
 */

package userfund

import (
	"context"
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
	operatorName := resolveOperatorName(l.ctx, l.svcCtx, req.UserStrId)
	chargeType := req.ChargeType
	if chargeType <= 0 {
		chargeType = 1
	}

	// 1. 先获取用户当前最新余额（可能是今天或之前的记录）
	currentBalance, err := l.svcCtx.UserBalanceDailyModel.GetLatestBalance(l.ctx, req.UserStrId)
	if err != nil {
		l.Logger.Errorf("Failed to get current balance: %v", err)
		// 如果获取失败，默认使用 0
		currentBalance = 0
	}

	// 2. 插入充值记录
	insertQuery := `
		INSERT INTO ae_user_recharge_record (
			user_str_id,
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
	`
	now := time.Now()
	_, err = l.svcCtx.DB.ExecCtx(l.ctx, insertQuery, req.UserStrId, req.Amount, now, now, now, chargeType, req.Remarks, operatorName)
	if err != nil {
		l.Logger.Errorf("Failed to insert recharge record: %v", err)
		return &types.ManualRechargeResp{
			Success: false,
			Message: "充值记录插入失败",
		}, nil
	}

	// 3. 更新日余额统计（使用 ON CONFLICT DO UPDATE）
	// 关键修复：如果当天没有记录，INSERT 时使用 当前余额 + 充值金额
	// 如果当天有记录，UPDATE 时使用 当天余额 + 充值金额
	updateQuery := `
		INSERT INTO ae_user_balance_statistic_daily (user_str_id, day, balance, create_time, update_time)
		VALUES ($1, CURRENT_DATE, $2, $3, $4)
		ON CONFLICT (user_str_id, day)
		DO UPDATE SET
			balance = ae_user_balance_statistic_daily.balance + $5,
			update_time = $4
		RETURNING balance
	`
	// 计算新余额：当前余额 + 充值金额（用于 INSERT，如果当天没有记录）
	newBalanceForInsert := currentBalance + req.Amount
	var newBalance float64
	err = l.svcCtx.DB.QueryRowCtx(l.ctx, &newBalance, updateQuery, req.UserStrId, newBalanceForInsert, now, now, req.Amount)
	if err != nil {
		l.Logger.Errorf("Failed to update balance: %v", err)
		return &types.ManualRechargeResp{
			Success: false,
			Message: "余额更新失败",
		}, nil
	}

	return &types.ManualRechargeResp{
		Success:    true,
		Message:    "充值成功",
		NewBalance: newBalance,
	}, nil
}
