/*
 * 文件名称: getUserListLogic.go
 * 功能说明: 获取用户列表（含分页、搜索、日均消费计算）
 *
 * 主要功能:
 * - 用户列表分页查询
 * - 用户名/邮箱搜索
 * - 自动获取每个用户的最新余额（从日余额统计表）
 * - 自动计算每个用户的日均消费（最近30天平均值）
 *
 * 数据来源:
 * - mcp_user: 用户基础信息
 * - ae_user_balance_statistic_daily: 最新余额
 * - ae_user_consumption_record_daily: 日均消费计算（AVG(xlcredit_consume), xlcredit_consume > 0）
 *
 * API路由: GET /manager/api/userfund/list?page=1&pageSize=20&search=xxx
 */

package userfund

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserListLogic {
	return &GetUserListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserListLogic) GetUserList(req *types.UserListReq) (resp *types.UserListResp, err error) {
	users, total, err := l.svcCtx.McpUserModel.FindList(l.ctx, req.Page, req.PageSize, req.Search, "")
	if err != nil {
		l.Logger.Errorf("Failed to get user list: %v", err)
		return nil, err
	}

	var list []types.UserItem
	for _, u := range users {
		// 优先从日余额统计表中获取最新余额
		balance, err := l.svcCtx.UserBalanceDailyModel.GetLatestBalance(l.ctx, u.UserStrId)
		if err != nil {
			l.Logger.Errorf("Failed to get latest balance for user %s: %v", u.UserStrId, err)
			balance = 0
		} else {
			// 调试日志：打印查询到的余额（如果为0也打印，方便排查）
			if balance == 0 {
				l.Logger.Infof("User %s (username: %s) balance is 0 from daily table", u.UserStrId, u.Username)
			} else {
				l.Logger.Infof("User %s (username: %s) balance: %.2f", u.UserStrId, u.Username, balance)
			}
		}

		lastLogin := ""
		if !u.LastLoginAt.IsZero() {
			lastLogin = u.LastLoginAt.Format(time.RFC3339)
		}

		createTime := ""
		if !u.CreateTime.IsZero() {
			createTime = u.CreateTime.Format(time.RFC3339)
		}

		// 调试日志：打印邮箱数据
		l.Logger.Infof("User %s (username: %s) email: [%s]", u.UserStrId, u.Username, u.Email)

		// 计算日均消费（最近30天）
		dailyConsumption := 0.0
		consumptionQuery := `
			SELECT COALESCE(AVG(xlcredit_consume), 0) as avg_consumption
			FROM ae_user_consumption_record_daily
			WHERE user_str_id = $1
			AND day >= CURRENT_DATE - INTERVAL '30 days'
			AND xlcredit_consume > 0
		`
		err = l.svcCtx.DB.QueryRowCtx(l.ctx, &dailyConsumption, consumptionQuery, u.UserStrId)
		if err != nil {
			l.Logger.Errorf("Failed to get daily consumption for user %s: %v", u.UserStrId, err)
			dailyConsumption = 0
		}

		list = append(list, types.UserItem{
			Id:               u.Id,
			UserStrId:        u.UserStrId,
			Username:         u.Username,
			Phone:            u.Phone,
			Email:            u.Email,
			AvatarUrl:        u.AvatarUrl,
			Status:           u.Status,
			LastLoginAt:      lastLogin,
			CreateTime:       createTime,
			Balance:          balance,
			DailyConsumption: dailyConsumption,
		})
	}

	return &types.UserListResp{
		List:  list,
		Total: total,
	}, nil
}
