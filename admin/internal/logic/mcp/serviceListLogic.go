package mcp

import (
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/models"
	"AgentEarth-Mgr/models/mcp"
	"context"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceListLogic {
	return &ServiceListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceListLogic) ServiceList(req *types.ServiceListReq) (resp *types.BaseResp, err error) {
	listConditions := models.ListConditions{
		Conditions: []models.Condition{},
		Pages: models.Pages{
			Page: req.Page,
			Size: req.Size,
		},
		Sorts: []models.Sort{},
	}

	if len(req.Sort) > 0 {
		sortField := req.Sort
		if sortField == "price" {
			sortField = "xlcredit_price"
		}
		listConditions.Sorts = append(listConditions.Sorts, models.Sort{
			Filed: sortField,
			Order: req.Order,
		})
	} else {
		// 默认排序：可用优先，然后按 ID 倒序
		listConditions.Sorts = append(listConditions.Sorts, models.Sort{
			Filed: "enabled",
			Order: "desc",
		})
		listConditions.Sorts = append(listConditions.Sorts, models.Sort{
			Filed: "id",
			Order: "desc",
		})
	}
	if len(req.Search) > 0 {
		// Search logic is handled in GetListWithSearch
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
	list, total, err := l.svcCtx.McpServiceModel.GetListWithSearch(l.ctx, listConditions, req.Search, true)
	if err != nil {
		l.Logger.Errorf("GetListWithSearch error: %v", err)
		return
	}
	// 如果列表为空，返回空数组而不是 nil
	if list == nil {
		list = []*mcp.AeMcpServices{}
	}
	resp = &types.BaseResp{
		Code: 0,
		Data: types.D{
			"list":  list,
			"total": total,
		},
		Message: "success",
	}
	return
}
