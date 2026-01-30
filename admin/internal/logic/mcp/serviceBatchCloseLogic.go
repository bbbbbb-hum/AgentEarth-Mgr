package mcp

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceBatchCloseLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceBatchCloseLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceBatchCloseLogic {
	return &ServiceBatchCloseLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceBatchCloseLogic) ServiceBatchClose(req *types.ServiceBatchCloseReq) (resp *types.BaseResp, err error) {
	err = l.svcCtx.McpServiceModel.BatchClose(l.ctx, req.Ids)
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
