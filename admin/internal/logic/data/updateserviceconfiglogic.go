// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"context"
	"database/sql"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

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
	existingConfig, err := l.svcCtx.TaskNodeConfigModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "查询服务配置失败: " + err.Error(),
		}, nil
	}

	if req.Name != "" {
		existingConfig.Name = req.Name
	}

	if req.Type != "" {
		existingConfig.Type = req.Type
	}

	if req.Description != "" {
		existingConfig.Description = req.Description
	}

	if req.ProjectName != "" {
		existingConfig.ProjectName = req.ProjectName
	}

	if req.MaxInstance != 0 {
		existingConfig.MaxInstance = req.MaxInstance
	}

	if req.LaunchInfo != nil {
		existingConfig.LaunchInfo = *req.LaunchInfo
	}

	if req.ConnectInfo != nil {
		existingConfig.ConnectInfo = *req.ConnectInfo
	}

	if req.InstallInfo != nil {
		existingConfig.InstallInfo = sql.NullString{String: *req.InstallInfo, Valid: true}
	}

	if req.AccountRequired != nil {
		existingConfig.AccountRequired = sql.NullInt64{Int64: *req.AccountRequired, Valid: true}
	}

	if req.TestStatus != nil {
		existingConfig.TestStatus = sql.NullInt64{Int64: *req.TestStatus, Valid: true}
	}

	if req.OnlineStatus != nil {
		existingConfig.OnlineStatus = sql.NullInt64{Int64: *req.OnlineStatus, Valid: true}
	}

	err = l.svcCtx.TaskNodeConfigModel.Update(l.ctx, existingConfig)
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
