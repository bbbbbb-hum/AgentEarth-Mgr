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

/**
*
* 单条更新服务价格
*
 */

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

	oldPrice := mcpService.Price
	mcpService.Price = float64(req.Price)
	mcpService.UpdateTime = time.Now()

	err = l.svcCtx.McpServiceModel.Update(l.ctx, mcpService)
	if err != nil {
		return nil, err
	}

	// Audit Log
	operatorId := l.ctx.Value("userId")
	l.Logger.Infow("Price Modification Audit Log",
		logx.Field("type", "AUDIT_LOG"),
		logx.Field("action", "SINGLE_UPDATE"),
		logx.Field("operator_id", operatorId),
		logx.Field("mcp_id", req.ServerId),
		logx.Field("old_price", oldPrice),
		logx.Field("new_price", req.Price),
	)

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
