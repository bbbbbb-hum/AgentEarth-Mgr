package mcp

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceUpdateCreatedGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceUpdateCreatedGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceUpdateCreatedGroupLogic {
	return &ServiceUpdateCreatedGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceUpdateCreatedGroupLogic) ServiceUpdateCreatedGroup(req *types.ServiceUpdateCreatedGroupReq) (resp *types.BaseResp, err error) {
	err = l.svcCtx.McpServiceModel.UpdateCreatedGroup(l.ctx, req.Ids, req.CreatedGroup)
	if err != nil {
		return
	}
	resp = &types.BaseResp{
		Code:    0,
		Data:    nil,
		Message: "success",
	}
	return
}
