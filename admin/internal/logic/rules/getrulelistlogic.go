// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rules

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRuleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRuleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRuleListLogic {
	return &GetRuleListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRuleListLogic) GetRuleList(req *types.BaseListReq) (resp *types.RuleListResp, err error) {
	// todo: add your logic here and delete this line

	return
}
