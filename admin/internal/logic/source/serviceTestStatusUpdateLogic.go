package source

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceTestStatusUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceTestStatusUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceTestStatusUpdateLogic {
	return &ServiceTestStatusUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceTestStatusUpdateLogic) ServiceTestStatusUpdate(req *types.ServiceTestStatusUpdateReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	err = l.svcCtx.ExternalMcpServicesModel.UpdateTestStatus(l.ctx, req.Ids, req.TestStatus)
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
