// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package mcp

import (
	"context"

	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceConfigAccountCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceConfigAccountCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceConfigAccountCreateLogic {
	return &ServiceConfigAccountCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceConfigAccountCreateLogic) ServiceConfigAccountCreate(req *types.ServiceConfigAccountCreateReq) (resp *types.BaseResp, err error) {
	account := &configModel.AeMcpExternalServicesAccount{
		Name:     req.Name,
		AuthInfo: req.AuthInfo,
		ConfigId: req.ConfigId,
		Status:   req.Status,
	}

	_, err = l.svcCtx.TaskNodeConfigAccountModel.Insert(l.ctx, account)
	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "创建账号失败: " + err.Error(),
		}, nil
	}

	return &types.BaseResp{
		Code:    0,
		Message: "创建账号成功",
		Data: types.D{
			"service_id": req.ConfigId,
		},
	}, nil
}
