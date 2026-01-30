package mcp

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceConfigAccountDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceConfigAccountDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceConfigAccountDeleteLogic {
	return &ServiceConfigAccountDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceConfigAccountDeleteLogic) ServiceConfigAccountDelete(req *types.ServiceConfigAccountDeleteReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	err = l.svcCtx.TaskNodeConfigAccountModel.BatchDelete(l.ctx, req.Ids)
	if err != nil {
		return
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "删除成功",
		Data:    nil,
	}
	return
}
