package mcp

import (
	"AgentEarth-Mgr/models"
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceInstallListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceInstallListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceInstallListLogic {
	return &ServiceInstallListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceInstallListLogic) ServiceInstallList(req *types.ServiceInstallListReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	list, total, err := l.svcCtx.McpServicesInstallModel.GetList(l.ctx, models.ListConditions{
		Pages: models.Pages{
			Page: req.Page,
			Size: req.Size,
		},
		Conditions: nil,
		Sorts:      nil,
	}, true)
	if err != nil {
		return
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"list":  list,
			"total": total,
		},
	}
	return
}
