package mcp

import (
	"AgentEarth-Mgr/models"
	"context"
	"errors"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceUpdateEnabledLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceUpdateEnabledLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceUpdateEnabledLogic {
	return &ServiceUpdateEnabledLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceUpdateEnabledLogic) ServiceUpdateEnabled(req *types.ServiceUpdateEnabledReq) (resp *types.BaseResp, err error) {
	mcpService, err := l.svcCtx.McpServiceModel.FindOneByCondition(l.ctx, []models.Condition{
		{
			Field: "id",
			Value: req.Id,
		},
	})
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return &types.BaseResp{
				Code:    -1,
				Message: "服务不存在",
			}, nil
		}
		return nil, err
	}

	mcpService.Enabled = req.Enabled
	mcpService.UpdateTime = time.Now()
	if err = l.svcCtx.McpServiceModel.Update(l.ctx, mcpService); err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "更新服务状态失败: " + err.Error(),
		}, nil
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"id":      req.Id,
			"enabled": req.Enabled,
		},
	}, nil
}

