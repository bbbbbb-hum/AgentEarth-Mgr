// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package mcp

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceConfigAccountUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceConfigAccountUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceConfigAccountUpdateLogic {
	return &ServiceConfigAccountUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceConfigAccountUpdateLogic) ServiceConfigAccountUpdate(req *types.ServiceConfigAccountUpdateReq) (resp *types.BaseResp, err error) {
	existingAccount, err := l.svcCtx.TaskNodeConfigAccountModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "查询账号失败: " + err.Error(),
		}, nil
	}

	existingAccount.Name = req.Name
	existingAccount.Status = req.Status

	if req.AuthInfo != nil {
		existingAccount.AuthInfo = *req.AuthInfo
	}

	if req.ConfigId != nil {
		existingAccount.ConfigId = *req.ConfigId
	}

	err = l.svcCtx.TaskNodeConfigAccountModel.Update(l.ctx, existingAccount)
	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "更新账号失败: " + err.Error(),
		}, nil
	}

	return &types.BaseResp{
		Code:    0,
		Message: "更新账号成功",
	}, nil
}
