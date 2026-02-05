// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"AgentEarth-Mgr/models"
	"context"
	"strconv"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetServiceConfigListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetServiceConfigListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetServiceConfigListLogic {
	return &GetServiceConfigListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetServiceConfigListLogic) GetServiceConfigList(req *types.GetServiceConfigListReq) (resp *types.BaseResp, err error) {
	var conditions []models.Condition

	if len(req.Search) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "name",
			Symbol: "ILIKE",
			Value:  "%" + req.Search + "%",
		})
	}

	if len(req.FilterName) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "name",
			Symbol: "ILIKE",
			Value:  "%" + req.FilterName + "%",
		})
	}

	if len(req.FilterWemcpName) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "wemcp_name",
			Symbol: "ILIKE",
			Value:  "%" + req.FilterWemcpName + "%",
		})
	}

	if len(req.FilterAccountRequired) > 0 {
		if v, parseErr := strconv.ParseInt(req.FilterAccountRequired, 10, 64); parseErr == nil {
			conditions = append(conditions, models.Condition{
				Field:  "account_required",
				Symbol: "=",
				Value:  v,
			})
		}
	}

	if len(req.FilterTestStatus) > 0 {
		if v, parseErr := strconv.ParseInt(req.FilterTestStatus, 10, 64); parseErr == nil {
			conditions = append(conditions, models.Condition{
				Field:  "test_status",
				Symbol: "=",
				Value:  v,
			})
		}
	}

	if len(req.FilterOnlineStatus) > 0 {
		if v, parseErr := strconv.ParseInt(req.FilterOnlineStatus, 10, 64); parseErr == nil {
			conditions = append(conditions, models.Condition{
				Field:  "online_status",
				Symbol: "=",
				Value:  v,
			})
		}
	}

	var orderBy string
	if len(req.SortField) > 0 {
		field := req.SortField
		if field == "accountRequired" {
			field = "account_required"
		}
		if field == "wemcpName" {
			field = "wemcp_name"
		}
		if field == "testStatus" {
			field = "test_status"
		}
		if field == "onlineStatus" {
			field = "online_status"
		}

		order := "ASC"
		if req.SortOrder == "desc" {
			order = "DESC"
		}
		orderBy = field + " " + order
	}

	list, total, err := l.svcCtx.TaskNodeConfigV2Model.GetList(l.ctx, models.ListConditions{
		Conditions: conditions,
		Pages: models.Pages{
			Page: req.Page,
			Size: req.Size,
		},
		OrderBy: orderBy,
	}, true)

	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "获取服务配置列表失败: " + err.Error(),
		}, nil
	}

	var items []types.ServiceConfigV2Item
	for _, item := range list {
		items = append(items, types.ServiceConfigV2Item{
			Id:              item.Id,
			Name:            item.Name,
			WemcpName:       item.WemcpName,
			Tags:            []string(item.Tags),
			Description:     item.Description,
			Comments:        item.Comments,
			CodeSourceUrl:   item.CodeSourceUrl,
			AccountRequired: item.AccountRequired,
			TestStatus:      item.TestStatus,
			OnlineStatus:    item.OnlineStatus,
			CreateTime:      item.CreateTime.Format("2006-01-02 15:04:05"),
			UpdateTime:      item.UpdateTime.Format("2006-01-02 15:04:05"),
		})
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"list":  items,
			"total": total,
		},
	}, nil
}
