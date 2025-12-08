package source

import (
	"AgentEarth-Mgr/models"
	"AgentEarth-Mgr/models/external"
	"context"
	"errors"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceOfflineLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceOfflineLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceOfflineLogic {
	return &ServiceOfflineLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceOfflineLogic) ServiceOffline(req *types.ServiceOfflineReq) (resp *types.BaseResp, err error) {
	// 获取外部配置列表
	externalMcpServices, total, err := l.svcCtx.ExternalMcpServicesModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field:  "id",
				Symbol: "in",
				Value:  req.Ids,
			},
		},
	}, true)
	if err != nil {
		return nil, err
	}
	var dealTotal int64
	if total > 0 {
		dealTotal, err = l.deal(externalMcpServices)
		if err != nil {
			return nil, err
		}
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"total":      total,
			"deal_total": dealTotal,
		},
	}

	return
}

func (l *ServiceOfflineLogic) deal(externalMcpServices []*external.ExternalMcpServices) (total int64, err error) {
	for _, ems := range externalMcpServices {
		mcpService, err1 := l.svcCtx.McpServiceModel.FindOneByCondition(l.ctx, []models.Condition{
			{
				Field: "server_name",
				Value: ems.ServerName,
			},
		})
		if err1 != nil && !errors.Is(err1, sqlx.ErrNotFound) {
			err = err1
			return
		}
		if mcpService == nil {
			logx.Error("未找到MCP服务：", ems.ServerName)
			continue
		}
		mcpService.Enabled = false
		mcpService.IsCreated = false
		mcpService.UpdateTime = time.Now()
		err = l.svcCtx.McpServiceModel.Update(l.ctx, mcpService)
		if err != nil {
			logx.Error("更新错误：", ems.ServerName, err)
			continue
		}
		logx.Info("下线成功：", ems.ServerName)
		total++
	}
	return
}
