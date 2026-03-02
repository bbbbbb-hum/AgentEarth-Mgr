package rules

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// RuleDashboardLogic 规则仪表盘：运行中规则数、本月触达用户、本月自动赠送点数
type RuleDashboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRuleDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RuleDashboardLogic {
	return &RuleDashboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RuleDashboardLogic) GetRuleDashboard() (resp *types.RuleDashboardResp, err error) {
	runningRules, monthlyTouchedUsers, monthlyAutoPoints, err := l.svcCtx.RuleQueryModel.GetRuleDashboard(l.ctx)
	if err != nil {
		return nil, err
	}

	return &types.RuleDashboardResp{
		RunningRules:        runningRules,
		MonthlyTouchedUsers: monthlyTouchedUsers,
		MonthlyAutoPoints:   monthlyAutoPoints,
	}, nil
}
