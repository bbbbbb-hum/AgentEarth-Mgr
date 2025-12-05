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

type ServiceUpdateIsCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceUpdateIsCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceUpdateIsCreateLogic {
	return &ServiceUpdateIsCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceUpdateIsCreateLogic) ServiceUpdateIsCreate(req *types.ServiceUpdateIsCreateReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	mcpService, err := l.svcCtx.McpServiceModel.FindOneByCondition(l.ctx, []models.Condition{
		{
			Field: "id",
			Value: req.Id,
		},
	})
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return
	}
	mcpService.IsCreated = req.IsCreated
	mcpService.UpdateTime = time.Now()
	err = l.svcCtx.McpServiceModel.Update(l.ctx, mcpService)
	if err != nil {
		return
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"id": req.Id,
		},
	}
	return
}
