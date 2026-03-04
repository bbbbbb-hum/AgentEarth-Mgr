// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rules

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ToggleRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewToggleRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleRuleLogic {
	return &ToggleRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ToggleRuleLogic) ToggleRule(req *types.ToggleRuleReq) error {
	rule, err := l.svcCtx.RuleModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return err
	}

	if req.IsActive {
		rule.Active = "active"
	} else {
		rule.Active = "inactive"
	}
	rule.UpdateTime = time.Now()

	if err := l.svcCtx.RuleModel.Update(l.ctx, rule); err != nil {
		l.Logger.Errorf("启用/停用规则失败, id=%d, isActive=%v, err=%v", req.Id, req.IsActive, err)
		return err
	}

	// 同步调度器：启用则注册/更新，停用则移除。
	if rule.Active == "active" {
		registerRuleInScheduler(l.ctx, rule.Id, rule.CronValue)
	} else {
		unregisterRuleFromScheduler(l.ctx, rule.Id)
	}

	return nil
}
