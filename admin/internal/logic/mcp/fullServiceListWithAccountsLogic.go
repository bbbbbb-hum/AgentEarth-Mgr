package mcp

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FullServiceListWithAccountsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFullServiceListWithAccountsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FullServiceListWithAccountsLogic {
	return &FullServiceListWithAccountsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FullServiceListWithAccountsLogic) FullServiceListWithAccounts(req *types.ServiceListReq) (resp *types.BaseResp, err error) {
	// 1. 获取所有服务列表 (如果 req.Size 为 0 则获取全部，但为了安全这里给个较大的默认值或由请求决定)
	// 如果是给 wrapper 用，通常需要全量数据
	listConditions := models.ListConditions{
		Conditions: []models.Condition{},
		Pages: models.Pages{
			Page: req.Page,
			Size: req.Size,
		},
		Sorts: []models.Sort{
			{
				Filed: "id",
				Order: "asc",
			},
		},
	}

	// 支持一些基础过滤
	if len(req.Search) > 0 {
		listConditions.Conditions = append(listConditions.Conditions, models.Condition{
			Field:  "server_name",
			Symbol: "ILIKE",
			Value:  "%" + req.Search + "%",
		})
	}
	if req.Enabled != 0 {
		listConditions.Conditions = append(listConditions.Conditions, models.Condition{
			Field:  "enabled",
			Symbol: "=",
			Value:  req.Enabled == 1,
		})
	}

	services, total, err := l.svcCtx.McpServiceModel.GetList(l.ctx, listConditions, true)
	if err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return &types.BaseResp{
			Code: 0,
			Data: types.D{
				"list":  []interface{}{},
				"total": total,
			},
			Message: "success",
		}, nil
	}

	// 2. 收集所有 ServerId
	serverIds := make([]string, 0, len(services))
	for _, s := range services {
		serverIds = append(serverIds, s.ServerId)
	}

	// 3. 批量获取 Config
	configs, _, err := l.svcCtx.TaskNodeConfigModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field:  "server_id",
				Symbol: "in",
				Value:  serverIds,
			},
		},
	}, true)
	if err != nil {
		return nil, err
	}

	// 4. 收集所有 ConfigId 并建立 ServerId -> ConfigId 映射
	configIds := make([]int64, 0, len(configs))
	serverToConfigId := make(map[string]int64)
	for _, c := range configs {
		configIds = append(configIds, c.Id)
		serverToConfigId[c.ServerId] = c.Id
	}

	// 5. 批量获取 Accounts
	accounts := make([]*configModel.AeMcpExternalServicesAccount, 0)
	if len(configIds) > 0 {
		accounts, _, err = l.svcCtx.TaskNodeConfigAccountModel.GetList(l.ctx, models.ListConditions{
			Conditions: []models.Condition{
				{
					Field:  "config_id",
					Symbol: "in",
					Value:  configIds,
				},
			},
		}, true)
		if err != nil {
			return nil, err
		}
	}

	// 6. 建立 ConfigId -> Accounts 映射
	configToAccounts := make(map[int64][]*configModel.AeMcpExternalServicesAccount)
	for _, a := range accounts {
		configToAccounts[a.ConfigId] = append(configToAccounts[a.ConfigId], a)
	}

	// 7. 组装最终结果
	resultList := make([]map[string]interface{}, 0, len(services))
	for _, s := range services {
		item := map[string]interface{}{
			"service":      s,
			"account_list": []interface{}{},
		}
		if configId, ok := serverToConfigId[s.ServerId]; ok {
			if accList, ok := configToAccounts[configId]; ok {
				item["account_list"] = accList
			}
		}
		resultList = append(resultList, item)
	}

	resp = &types.BaseResp{
		Code: 0,
		Data: types.D{
			"list":  resultList,
			"total": total,
		},
		Message: "success",
	}

	return resp, nil
}
