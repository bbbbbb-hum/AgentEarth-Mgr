/*
 * 文件名称: manualRechargeLogic.go
 * 功能说明: 人工充值功能（管理员后台充值）
 *
 * 主要功能:
 * - 仅在 ae_user_recharge_record 表插入充值记录，不修改用户日余额统计表
 * - 日余额由专门统计系统更新，避免与统计系统重复加导致错误
 *
 * 数据操作:
 * - 插入充值记录: charge_source=2（运营手工单个操作）, xlcredit_amount=充值金额, pay_time=当前时间
 *
 * API路由: POST /manager/api/userfund/user/recharge
 * 请求体: {"user_id": "xxx", "amount": 100.00, "remarks": "活动赠送"}
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
	// 充值类型直接使用新枚举（101/201/301）；<=0 时默认常规充值 101
	chargeType := req.ChargeType
	if chargeType <= 0 {
		chargeType = 101
	}

	// 1. 获取当前余额仅用于日志与返回值展示，不写入日余额表
	currentBalance, err := l.svcCtx.UserBalanceDailyModel.GetLatestBalance(l.ctx, req.UserId)
	if err != nil {
		l.Logger.Errorf("Failed to get current balance: %v", err)
		currentBalance = 0
	}

	// 2. 解析可选过期时间（格式 YYYY-MM-DD，不传或空为“永久有效”），
	// 但在数据库中用 9999-12-31 23:59:59 作为“永不过期”的哨兵时间，避免与 NULL（无意义）混淆。
	var expireTime sql.NullTime
	rawExpire := strings.TrimSpace(req.ExpireTime)
	if len(rawExpire) > 0 {
		if t, parseErr := time.ParseInLocation("2006-01-02", rawExpire, time.Local); parseErr == nil {
			// 设为该日 23:59:59，表示该日结束前有效
			endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
			expireTime = sql.NullTime{Time: endOfDay, Valid: true}
		}
	} else {
		// 未指定过期时间：写入永不过期哨兵时间。
		never := time.Date(9999, 12, 31, 23, 59, 59, 0, time.Local)
		expireTime = sql.NullTime{Time: never, Valid: true}
	}

	// 3. 插入充值记录（含可选 expire_time），不修改日余额统计表，SQL 在 model 层
	now := time.Now()
	insertedId, err := l.svcCtx.UserRechargeRecordModel.InsertRechargeRecordWithExpire(l.ctx, req.UserId, req.Amount, now, now, now, chargeType, req.Remarks, operatorName, expireTime)
	if err != nil {
		l.Logger.Errorf("Failed to insert recharge record (full): %v, trying fallback", err)
		insertedId, err = l.svcCtx.UserRechargeRecordModel.InsertRechargeRecordWithoutExpire(l.ctx, req.UserId, req.Amount, now, now, now, chargeType, req.Remarks, operatorName)
		if err != nil {
			l.Logger.Errorf("Failed to insert recharge record (fallback): %v", err)
			return &types.ManualRechargeResp{
				Success: false,
				Message: "充值记录插入失败",
			}, nil
		}
	}
	l.Logger.Infof("充值记录插入成功: record_id=%d user_id=%s amount=%.2f expire_time=%v", insertedId, req.UserId, req.Amount, expireTime)

	// 不修改日余额统计表，由统计系统统一更新，避免重复加导致错误
	newBalanceForDisplay := currentBalance + req.Amount
	l.Logger.Infof("管理员资金操作: 操作管理员=%s, 操作用户=%s, user_id=%s, 类型=充值, 时间=%s, 原金额=%.2f, 变动金额=%.2f, 预估余额=%.2f (日余额由统计系统更新)",
		operatorName, targetUsername, req.UserId, now.Format(time.RFC3339), currentBalance, req.Amount, newBalanceForDisplay)

	return &types.ManualRechargeResp{
		Success:    true,
		Message:    "充值成功，余额由统计系统更新",
		NewBalance: newBalanceForDisplay,
	}, nil
}
