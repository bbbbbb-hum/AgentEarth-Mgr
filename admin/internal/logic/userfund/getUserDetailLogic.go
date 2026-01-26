/*
 * 文件名称: getUserDetailLogic.go
 * 功能说明: 获取用户详情（用于用户详情页）
 *
 * 主要功能:
 * - 查询用户基础信息
 * - 获取用户当前最新余额（从日余额统计表）
 * - 计算用户日均消费（最近30天平均值）
 * - 计算资金续航天数 = 当前余额 / 日均消费
 *
 * API路由: GET /manager/api/userfund/user/:user_str_id
 */
package userfund

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type GetUserDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserDetailLogic {
	return &GetUserDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserDetailLogic) GetUserDetail(req *types.UserDetailReq) (resp *types.UserDetailResp, err error) {
	user, err := l.svcCtx.McpUserModel.FindOneByUserStrId(l.ctx, req.UserStrId)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, err
		}
		l.Logger.Errorf("Failed to get user detail: %v", err)
		return nil, err
	}

	// 获取最新余额（从日余额统计表）
	currentBalance, err := l.svcCtx.UserBalanceDailyModel.GetLatestBalance(l.ctx, req.UserStrId)
	if err != nil {
		l.Logger.Errorf("Failed to get latest balance for user %s: %v", req.UserStrId, err)
		currentBalance = 0
	}

	// 计算日均消费（最近30天）
	dailyConsumption := 0.0
	consumptionQuery := `
		SELECT COALESCE(AVG(xlcredit_consume), 0) as avg_consumption
		FROM ae_user_consumption_record_daily
		WHERE user_str_id = $1
		AND day >= CURRENT_DATE - INTERVAL '30 days'
		AND xlcredit_consume > 0
	`
	err = l.svcCtx.DB.QueryRowCtx(l.ctx, &dailyConsumption, consumptionQuery, req.UserStrId)
	if err != nil {
		l.Logger.Errorf("Failed to get daily consumption for user %s: %v", req.UserStrId, err)
		dailyConsumption = 0
	}

	// 资金续航天数
	fundRunway := int64(9999)
	if dailyConsumption > 0 {
		fundRunway = int64(currentBalance / dailyConsumption)
	}

	lastLogin := ""
	if !user.LastLoginAt.IsZero() {
		lastLogin = user.LastLoginAt.Format(time.RFC3339)
	}
	createTime := ""
	if !user.CreateTime.IsZero() {
		createTime = user.CreateTime.Format(time.RFC3339)
	}

	return &types.UserDetailResp{
		User: types.UserItem{
			Id:               user.Id,
			UserStrId:        user.UserStrId,
			Username:         user.Username,
			Phone:            user.Phone,
			Email:            user.Email,
			AvatarUrl:        user.AvatarUrl,
			Status:           user.Status,
			LastLoginAt:      lastLogin,
			CreateTime:       createTime,
			Balance:          currentBalance,
			DailyConsumption: dailyConsumption,
		},
		CurrentBalance:   currentBalance,
		DailyConsumption: dailyConsumption,
		FundRunway:       fundRunway,
	}, nil
}
