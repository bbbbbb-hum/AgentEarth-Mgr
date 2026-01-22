package mcp

import (
	"AgentEarth-Mgr/models"
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

/**
*
* 批量更新服务价格
*
 */

type ServiceBatchUpdatePriceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceBatchUpdatePriceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceBatchUpdatePriceLogic {
	return &ServiceBatchUpdatePriceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceBatchUpdatePriceLogic) ServiceBatchUpdatePrice(req *types.ServiceBatchUpdatePriceReq) (resp *types.BaseResp, err error) {
	var count int64
	if req.IsAll {
		// Global update mode
		listConditions := models.ListConditions{
			Conditions: []models.Condition{},
		}

		if req.Enabled != 0 {
			var enabled bool
			if req.Enabled == 1 {
				enabled = true
			}
			listConditions.Conditions = append(listConditions.Conditions, models.Condition{
				Field:  "enabled",
				Symbol: "=",
				Value:  enabled,
			})
		}
		if req.IsInstall != 0 {
			var isInstall bool
			if req.IsInstall == 1 {
				isInstall = true
			}
			listConditions.Conditions = append(listConditions.Conditions, models.Condition{
				Field:  "is_install",
				Symbol: "=",
				Value:  isInstall,
			})
		}
		if req.IsCreated != 0 {
			var isCreated bool
			if req.IsCreated == 1 {
				isCreated = true
			}
			listConditions.Conditions = append(listConditions.Conditions, models.Condition{
				Field:  "is_created",
				Symbol: "=",
				Value:  isCreated,
			})
		}
		if len(req.ServerId) > 0 {
			listConditions.Conditions = append(listConditions.Conditions, models.Condition{
				Field:  "server_id",
				Symbol: "=",
				Value:  req.ServerId,
			})
		}

		count, err = l.svcCtx.McpServiceModel.BatchUpdatePriceByCondition(l.ctx, listConditions, req.Search, req.Price)
	} else {
		// ID list mode
		count, err = l.svcCtx.McpServiceModel.BatchUpdatePrice(l.ctx, req.Ids, req.Price)
	}

	if err != nil {
		return nil, err
	}

	// Audit Log
	operatorId := l.ctx.Value("userId")
	if req.IsAll {
		l.Logger.Infow("Price Modification Audit Log",
			logx.Field("type", "AUDIT_LOG"),
			logx.Field("action", "BATCH_UPDATE"),
			logx.Field("operator_id", operatorId),
			logx.Field("new_price", req.Price),
			logx.Field("count", count),
			logx.Field("filter_conditions", map[string]interface{}{
				"enabled":    req.Enabled,
				"is_install": req.IsInstall,
				"is_created": req.IsCreated,
				"server_id":  req.ServerId,
				"search":     req.Search,
			}),
		)
	} else {
		l.Logger.Infow("Price Modification Audit Log",
			logx.Field("type", "AUDIT_LOG"),
			logx.Field("action", "BATCH_UPDATE"),
			logx.Field("operator_id", operatorId),
			logx.Field("new_price", req.Price),
			logx.Field("count", count),
			logx.Field("target_ids", req.Ids),
		)
	}

	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    nil,
	}
	return
}
