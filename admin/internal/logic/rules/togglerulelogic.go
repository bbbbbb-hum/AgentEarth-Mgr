// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rules

import (
	"context"

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
	// todo: add your logic here and delete this line

	return nil
}
