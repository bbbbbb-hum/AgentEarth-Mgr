/*
 * 文件名称: getStatsLogic.go
 * 功能说明: 获取用户资金管理统计数据
 *
 * 主要功能:
 * - 总注册用户数统计
 * - 今日活跃用户数统计
 * - 24小时充值总额统计（只统计正数充值，排除扣减）
 *
 * 数据来源:
 * - mcp_user: 总用户数、今日活跃用户
 * - ae_user_recharge_record: 24小时充值金额（xlcredit_amount > 0）
 *
 * API路由: GET /manager/api/userfund/stats
 */

package userfund

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStatsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStatsLogic {
	return &GetStatsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetStatsLogic) GetStats() (resp *types.UserFundStatsResp, err error) {
	totalUsers, err := l.svcCtx.McpUserModel.CountTotal(l.ctx)
	if err != nil {
		l.Logger.Errorf("Failed to count total users: %v", err)
		return nil, err
	}
	l.Logger.Infof("Total users count: %d", totalUsers)

	dailyActive, err := l.svcCtx.McpUserModel.CountTodayActive(l.ctx)
	if err != nil {
		l.Logger.Errorf("Failed to count daily active users: %v", err)
		return nil, err
	}
	l.Logger.Infof("Daily active users count: %d (querying users with last_login_at >= CURRENT_DATE)", dailyActive)

	// 如果统计为0，记录信息（可能是正常的，如果没有用户今天登录）
	if dailyActive == 0 {
		l.Logger.Infof("Daily active count is 0, this might be correct if no users logged in today")
	}

	recharge24h, err := l.svcCtx.UserRechargeRecordModel.Sum24hRecharge(l.ctx)
	if err != nil {
		l.Logger.Errorf("Failed to sum 24h recharge: %v", err)
		return nil, err
	}
	l.Logger.Infof("24h recharge total: %.2f (from ae_user_recharge_record table, pay_time >= NOW() - 24 HOURS, xlcredit_amount > 0)", recharge24h)

	return &types.UserFundStatsResp{
		TotalUsers:       totalUsers,
		DailyActiveUsers: dailyActive,
		TotalRecharge24h: recharge24h,
	}, nil
}
