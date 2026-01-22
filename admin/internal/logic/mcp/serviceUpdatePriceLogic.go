package mcp

import (
	"context"
	"errors"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceUpdatePriceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceUpdatePriceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceUpdatePriceLogic {
	return &ServiceUpdatePriceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceUpdatePriceLogic) ServiceUpdatePrice(req *types.ServiceUpdatePriceReq) (resp *types.BaseResp, err error) {
	mcpService, err := l.svcCtx.McpServiceModel.FindOne(l.ctx, req.ServerId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return &types.BaseResp{
				Code:    -1,
				Message: "service not found",
				Data:    nil,
			}, nil
		}
		return nil, err
	}

	mcpService.Price = req.Price
	mcpService.UpdateTime = time.Now()

	err = l.svcCtx.McpServiceModel.Update(l.ctx, mcpService)
	if err != nil {
		return nil, err
	}

	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"server_id": req.ServerId,
			"price":     req.Price,
		},
	}
	return
}
