// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"context"
	"errors"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/models"
	"AgentEarth-Mgr/models/mcp"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type UpdateServiceOnlineLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateServiceOnlineLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateServiceOnlineLogic {
	return &UpdateServiceOnlineLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateServiceOnlineLogic) UpdateServiceOnline(req *types.UpdateServiceOnlineReq) (resp *types.BaseResp, err error) {
	service, err := l.svcCtx.TaskNodeConfigModel.FindOne(l.ctx, req.Id)
	if err != nil {
		logx.Errorf("Failed to find service config: %v", err)
		return &types.BaseResp{
			Code:    -1,
			Message: "服务不存在",
		}, nil
	}

	service.OnlineStatus.Valid = true
	service.OnlineStatus.Int64 = req.OnlineStatus

	err = l.svcCtx.TaskNodeConfigModel.Update(l.ctx, service)
	if err != nil {
		logx.Errorf("Failed to update service online status: %v", err)
		return &types.BaseResp{
			Code:    -1,
			Message: "更新服务上线状态失败: " + err.Error(),
		}, nil
	}

	var mcpService *mcp.AeMcpServices
	if len(service.ServerId) > 0 {
		found, err2 := l.svcCtx.McpServiceModel.FindOne(l.ctx, service.ServerId)
		if err2 != nil && !errors.Is(err2, sqlx.ErrNotFound) {
			return &types.BaseResp{
				Code:    -1,
				Message: "查询服务信息失败: " + err2.Error(),
			}, nil
		}
		if err2 == nil {
			mcpService = found
		}
	}
	if mcpService == nil {
		found, err2 := l.svcCtx.McpServiceModel.FindOneByCondition(l.ctx, []models.Condition{
			{
				Field:  "server_name",
				Symbol: "=",
				Value:  service.Name,
			},
		})
		if err2 != nil && !errors.Is(err2, sqlx.ErrNotFound) {
			return &types.BaseResp{
				Code:    -1,
				Message: "查询服务信息失败: " + err2.Error(),
			}, nil
		}
		if err2 == nil {
			mcpService = found
			if service.ServerId == "" {
				service.ServerId = found.ServerId
				if err3 := l.svcCtx.TaskNodeConfigModel.Update(l.ctx, service); err3 != nil {
					return &types.BaseResp{
						Code:    -1,
						Message: "补全服务ID失败: " + err3.Error(),
					}, nil
				}
			}
		}
	}
	if mcpService != nil {
		mcpService.Enabled = req.OnlineStatus == 1
		mcpService.UpdateTime = time.Now()
		if err3 := l.svcCtx.McpServiceModel.Update(l.ctx, mcpService); err3 != nil {
			return &types.BaseResp{
				Code:    -1,
				Message: "同步服务状态失败: " + err3.Error(),
			}, nil
		}
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
	}, nil
}
