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
 * 请求体: {"user_id": "xxx", "amount": 100.00, "remarks": "违规扣减"}
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
	logx.Logger                     //日志工具
	ctx         context.Context     //上下文(用于超时控制，链路追踪)
	svcCtx      *svc.ServiceContext //服务上下文(包含数据库连接、Redis、配置等资源)
}

// 构造函数（工厂方法），初始化上面的ManualDeductionLogic结构体
func NewManualDeductionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManualDeductionLogic {
	return &ManualDeductionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ManualDeductionLogic) ManualDeduction(req *types.ManualDeductionReq) (resp *types.ManualDeductionResp, err error) {
	// operatorName := resolveOperatorName(l.ctx, l.svcCtx, req.UserId, -1)
	operatorName := "System_Auto"
	targetUsername := resolveTargetUsername(l.ctx, l.svcCtx, req.UserId)
	chargeType := req.ChargeType
	if chargeType <= 0 {
		chargeType = 1
	}

	// 1. 获取用户当前最新余额（可能是今天或之前的记录）
	currentBalance, err := l.svcCtx.UserBalanceDailyModel.GetLatestBalance(l.ctx, req.UserId)
	if err != nil {
		l.Logger.Errorf("Failed to get current balance: %v", err)
		currentBalance = 0
	}

	// 2. 插入扣减记录（金额为负数）
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
			operator
		)
		VALUES ($1, $2, $3, $4, $5, -1, $6, $7, $8)
	`
	fallbackInsertQuery := `
		INSERT INTO public.ae_user_recharge_record (
			user_id,
			xlcredit_amount,
			pay_time,
			create_time,
			update_time,
			charge_source
		)
		VALUES ($1, $2, $3, $4, $5, -1)
	`
	now := time.Now()
	negativeAmount := -req.Amount
	_, err = l.svcCtx.DB.ExecCtx(l.ctx, insertQuery, req.UserId, negativeAmount, now, now, now, chargeType, req.Remarks, operatorName)
	if err != nil {
		l.Logger.Errorf("Failed to insert deduction record (full): %v, trying fallback", err)
		_, fallbackErr := l.svcCtx.DB.ExecCtx(l.ctx, fallbackInsertQuery, req.UserId, negativeAmount, now, now, now)
		if fallbackErr != nil {
			l.Logger.Errorf("Failed to insert deduction record (fallback): %v", fallbackErr)
			return &types.ManualDeductionResp{
				Success: false,
				Message: "扣减记录插入失败",
			}, nil
		}
	}

	// 3. 更新日余额统计（使用 ON CONFLICT DO UPDATE）
	updateQuery := `
		INSERT INTO ae_user_balance_statistic_daily (user_id, day, balance, create_time, update_time)
		VALUES ($1, CURRENT_DATE, $2, $3, $4)
		ON CONFLICT (user_id, day)
		DO UPDATE SET
			balance = ae_user_balance_statistic_daily.balance - $5,
			update_time = $4
		RETURNING balance
	`
	newBalanceForInsert := currentBalance - req.Amount
	var newBalance float64
	err = l.svcCtx.DB.QueryRowCtx(l.ctx, &newBalance, updateQuery, req.UserId, newBalanceForInsert, now, now, req.Amount)
	if err != nil {
		l.Logger.Errorf("Failed to update balance: %v", err)
		return &types.ManualDeductionResp{
			Success: false,
			Message: "余额更新失败",
		}, nil
	}

	l.Logger.Infof("管理员资金操作: 操作管理员=%s, 操作用户=%s, user_id=%s, 类型=扣减, 时间=%s, 原金额=%.2f, 变动金额=%.2f, 操作后金额=%.2f",
		operatorName, targetUsername, req.UserId, now.Format(time.RFC3339), currentBalance, req.Amount, newBalance)

	return &types.ManualDeductionResp{
		Success:    true,
		Message:    "扣减成功",
		NewBalance: newBalance,
	}, nil
}
