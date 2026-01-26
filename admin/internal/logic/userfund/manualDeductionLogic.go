/*
 * 文件名称: manualDeductionLogic.go
 * 功能说明: 人工扣减功能（管理员后台扣减）
 *
 * 主要功能:
 * - 记录扣减记录到 ae_user_recharge_record 表（金额为负数）
 * - 更新用户日余额统计表 ae_user_balance_statistic_daily
 * - 使用 ON CONFLICT DO UPDATE 确保同一天的余额记录唯一性
 * - 返回扣减后的新余额
 *
 * API路由: POST /manager/api/userfund/user/deduction
 * 请求体: {"user_str_id": "xxx", "amount": 100.00, "remarks": "违规扣减"}
 */
package userfund

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ManualDeductionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewManualDeductionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManualDeductionLogic {
	return &ManualDeductionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ManualDeductionLogic) ManualDeduction(req *types.ManualDeductionReq) (resp *types.ManualDeductionResp, err error) {
	operatorName := resolveOperatorName(l.ctx, l.svcCtx, req.UserStrId)
	chargeType := req.ChargeType
	if chargeType <= 0 {
		chargeType = 1
	}

	// 1. 获取用户当前最新余额（可能是今天或之前的记录）
	currentBalance, err := l.svcCtx.UserBalanceDailyModel.GetLatestBalance(l.ctx, req.UserStrId)
	if err != nil {
		l.Logger.Errorf("Failed to get current balance: %v", err)
		currentBalance = 0
	}

	// 2. 插入扣减记录（金额为负数）
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
		VALUES ($1, $2, $3, $4, $5, -1, $6, $7, $8)
	`
	now := time.Now()
	negativeAmount := -req.Amount
	_, err = l.svcCtx.DB.ExecCtx(l.ctx, insertQuery, req.UserStrId, negativeAmount, now, now, now, chargeType, req.Remarks, operatorName)
	if err != nil {
		l.Logger.Errorf("Failed to insert deduction record: %v", err)
		return &types.ManualDeductionResp{
			Success: false,
			Message: "扣减记录插入失败",
		}, nil
	}

	// 3. 更新日余额统计（使用 ON CONFLICT DO UPDATE）
	updateQuery := `
		INSERT INTO ae_user_balance_statistic_daily (user_str_id, day, balance, create_time, update_time)
		VALUES ($1, CURRENT_DATE, $2, $3, $4)
		ON CONFLICT (user_str_id, day)
		DO UPDATE SET
			balance = ae_user_balance_statistic_daily.balance - $5,
			update_time = $4
		RETURNING balance
	`
	newBalanceForInsert := currentBalance - req.Amount
	var newBalance float64
	err = l.svcCtx.DB.QueryRowCtx(l.ctx, &newBalance, updateQuery, req.UserStrId, newBalanceForInsert, now, now, req.Amount)
	if err != nil {
		l.Logger.Errorf("Failed to update balance: %v", err)
		return &types.ManualDeductionResp{
			Success: false,
			Message: "余额更新失败",
		}, nil
	}

	return &types.ManualDeductionResp{
		Success:    true,
		Message:    "扣减成功",
		NewBalance: newBalance,
	}, nil
}
