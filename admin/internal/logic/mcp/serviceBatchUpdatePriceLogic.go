package mcp

import (
	"AgentEarth-Mgr/models"
	"context"
	"fmt"

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
	// 1. Build Conditions for Querying Targets
	listConditions := models.ListConditions{
		Conditions: []models.Condition{},
	}
	var searchStr string

	if req.IsAll {
		searchStr = req.Search
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
	} else {
		// ID list mode
		if len(req.Ids) > 0 {
			listConditions.Conditions = append(listConditions.Conditions, models.Condition{
				Field:  "id",
				Symbol: "IN",
				Value:  req.Ids,
			})
		}
	}

	// 2. Fetch Targets to be updated (for Audit Log)
	// We want all matching records, so we don't set Pages (defaults to 0/0 -> no limit)
	targets, _, err := l.svcCtx.McpServiceModel.GetListWithSearch(l.ctx, listConditions, searchStr, true)
	if err != nil {
		l.Logger.Errorf("Failed to fetch targets for batch price update: %v", err)
		return nil, err
	}

	// 3. Extract IDs and Old Prices
	var targetIds []int64
	var oldPrices []float64
	for _, t := range targets {
		targetIds = append(targetIds, t.Id)
		oldPrices = append(oldPrices, t.Price)
	}

	// 4. Perform Update
	var count int64
	if req.IsAll {
		count, err = l.svcCtx.McpServiceModel.BatchUpdatePriceByCondition(l.ctx, listConditions, req.Search, req.Price)
	} else {
		count, err = l.svcCtx.McpServiceModel.BatchUpdatePrice(l.ctx, req.Ids, req.Price)
	}

	if err != nil {
		return nil, err
	}

	// 5. Audit Log with Username
	operatorId := l.ctx.Value("userId")
	var operatorName string
	operatorIdStr := fmt.Sprintf("%v", operatorId)

	if operatorIdStr != "" {
		user, err := l.svcCtx.UserModel.FindOneByUserId(l.ctx, operatorIdStr)
		if err == nil {
			operatorName = user.Username
		} else {
			operatorName = operatorIdStr
		}
	} else {
		operatorName = "unknown"
	}

	logFields := []logx.LogField{
		logx.Field("type", "AUDIT_LOG"),
		logx.Field("action", "BATCH_UPDATE"),
		logx.Field("operator_id", operatorName),
		logx.Field("new_price", req.Price),
		logx.Field("count", count),
		logx.Field("target_ids", targetIds),
		logx.Field("old_prices", oldPrices),
	}

	if req.IsAll {
		logFields = append(logFields, logx.Field("filter_conditions", map[string]interface{}{
			"enabled":    req.Enabled,
			"is_install": req.IsInstall,
			"is_created": req.IsCreated,
			"server_id":  req.ServerId,
			"search":     req.Search,
		}))
	}

	l.Logger.Infow("Price Modification Audit Log", logFields...)

	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    nil,
	}
	return
}
