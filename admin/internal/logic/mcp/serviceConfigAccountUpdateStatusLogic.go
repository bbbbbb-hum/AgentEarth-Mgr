package mcp

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceConfigAccountUpdateStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceConfigAccountUpdateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceConfigAccountUpdateStatusLogic {
	return &ServiceConfigAccountUpdateStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceConfigAccountUpdateStatusLogic) ServiceConfigAccountUpdateStatus(req *types.ServiceConfigAccountUpdateStatusReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	err = l.svcCtx.TaskNodeConfigAccountModel.BatchUpdateStatus(l.ctx, req.Ids, req.Status)
	if err != nil {
		return
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}
	return
}
