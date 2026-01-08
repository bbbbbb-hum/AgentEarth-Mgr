// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"AgentEarth-Mgr/models"
	"context"

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

	if len(req.FilterType) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "type",
			Symbol: "=",
			Value:  req.FilterType,
		})
	}

	if len(req.FilterAccountRequired) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "account_required",
			Symbol: "=",
			Value:  req.FilterAccountRequired,
		})
	}

	if len(req.FilterTestStatus) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "test_status",
			Symbol: "=",
			Value:  req.FilterTestStatus,
		})
	}

	if len(req.FilterOnlineStatus) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "online_status",
			Symbol: "=",
			Value:  req.FilterOnlineStatus,
		})
	}

	var orderBy string
	if len(req.SortField) > 0 {
		field := req.SortField
		if field == "accountRequired" {
			field = "account_required"
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

	list, total, err := l.svcCtx.TaskNodeConfigModel.GetList(l.ctx, models.ListConditions{
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

	var items []types.ServiceConfigItem
	for _, item := range list {
		accountRequired := int64(0)
		if item.AccountRequired.Valid {
			accountRequired = item.AccountRequired.Int64
		}

		testStatus := int64(0)
		if item.TestStatus.Valid {
			testStatus = item.TestStatus.Int64
		}

		onlineStatus := int64(0)
		if item.OnlineStatus.Valid {
			onlineStatus = item.OnlineStatus.Int64
		}

		items = append(items, types.ServiceConfigItem{
			Id:              item.Id,
			Name:            item.Name,
			Type:            item.Type,
			Description:     item.Description,
			ProjectName:     item.ProjectName,
			MaxInstance:     item.MaxInstance,
			LaunchInfo:      item.LaunchInfo,
			ConnectInfo:     item.ConnectInfo,
			InstallInfo:     item.InstallInfo.String,
			AccountRequired: accountRequired,
			TestStatus:      testStatus,
			OnlineStatus:    onlineStatus,
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
