// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateServiceConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateServiceConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateServiceConfigLogic {
	return &UpdateServiceConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateServiceConfigLogic) UpdateServiceConfig(req *types.UpdateServiceConfigReq) (resp *types.BaseResp, err error) {
	existingConfig, err := l.svcCtx.TaskNodeConfigV2Model.FindOne(l.ctx, req.Id)
	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "查询服务配置失败: " + err.Error(),
		}, nil
	}

	if req.Name != "" {
		existingConfig.Name = req.Name
	}

	if req.Description != "" {
		existingConfig.Description = req.Description
	}

	if req.WemcpName != nil && *req.WemcpName != "" {
		existingConfig.WemcpName = *req.WemcpName
	}

	if req.Tags != nil {
		existingConfig.Tags = pq.StringArray(*req.Tags)
	}

	if req.Comments != nil {
		existingConfig.Comments = *req.Comments
	}

	if req.CodeSourceUrl != nil {
		existingConfig.CodeSourceUrl = *req.CodeSourceUrl
	}

	if req.AccountRequired != nil {
		existingConfig.AccountRequired = *req.AccountRequired
	}

	if req.TestStatus != nil {
		existingConfig.TestStatus = *req.TestStatus
	}

	if req.OnlineStatus != nil {
		existingConfig.OnlineStatus = *req.OnlineStatus
	}

	err = l.svcCtx.TaskNodeConfigV2Model.Update(l.ctx, existingConfig)
	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "更新服务配置失败: " + err.Error(),
		}, nil
	}

	return &types.BaseResp{
		Code:    0,
		Message: "更新服务配置成功",
	}, nil
}
