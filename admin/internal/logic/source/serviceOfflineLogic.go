package source

import (
	"AgentEarth-Mgr/models"
	"AgentEarth-Mgr/models/external"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"errors"
	"fmt"
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
		err2 := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
			sessConn := sqlx.NewSqlConnFromSession(session)
			externalMcpServicesModel := external.NewExternalMcpServicesModel(sessConn)
			mcpServicesModel := mcp.NewAeMcpServicesModel(sessConn)
			installModel := mcp.NewAeMcpServicesInstallModel(sessConn)
			// 关闭MCP服务的启动状态和服务启动创建状态
			mcpService, err1 := mcpServicesModel.FindOneByCondition(ctx, []models.Condition{
				{
					Field: "server_name",
					Value: ems.ServerName,
				},
			})
			if err1 != nil && !errors.Is(err1, sqlx.ErrNotFound) {
				return err1
			}
			if mcpService == nil {
				return fmt.Errorf("未找到MCP服务：%s", ems.ServerName)
			}
			mcpService.Enabled = false
			mcpService.IsCreated = false
			mcpService.UpdateTime = time.Now()
			err1 = mcpServicesModel.Update(ctx, mcpService)
			if err1 != nil {
				return fmt.Errorf("更新错误：%s", err1.Error())
			}
			// 删除安装命令
			err1 = installModel.DeleteByConditions(ctx, []models.Condition{
				{
					Field: "server_id",
					Value: mcpService.ServerId,
				},
			})
			if err1 != nil {
				return fmt.Errorf("删除安装命令错误：%s", err1.Error())
			}
			// 更新源表数据状态
			ems.TestStatus = 11
			err1 = externalMcpServicesModel.Update(ctx, ems)
			if err1 != nil {
				return fmt.Errorf("更新源表数据状态错误：%s", err1.Error())
			}
			return nil
		})
		l.Info("下线成功：", ems.ServerName)
		if err2 != nil {
			l.Errorf("下线失败：%s", err2.Error())
			continue
		}
		total++
	}
	return
}
