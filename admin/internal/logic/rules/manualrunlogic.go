// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rules

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ManualRunLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewManualRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManualRunLogic {
	return &ManualRunLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ManualRunLogic) ManualRun(req *types.ManualRunReq) (resp *types.ManualRunResp, err error) {
	// todo: add your logic here and delete this line

	return
}
