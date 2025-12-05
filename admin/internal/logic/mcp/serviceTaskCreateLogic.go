package mcp

import (
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"errors"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceTaskCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceTaskCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceTaskCreateLogic {
	return &ServiceTaskCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceTaskCreateLogic) ServiceTaskCreate(req *types.ServiceTaskCreateReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	serviceList, total, err := l.svcCtx.McpServiceModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field:  "id",
				Symbol: "IN",
				Value:  req.ServiceIds,
			},
		},
	}, true)
	if err != nil {
		return
	}
	if total > 0 {
		go l.deal(serviceList)
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"total": total,
		},
	}
	return
}

func (l *ServiceTaskCreateLogic) deal(services []*mcp.AeMcpServices) {
	var ctx = context.Background()
	var num int64
	for _, service := range services {
		// one transaction per config
		err := l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			// session-scoped models via constructors bound to the session
			sessConn := sqlx.NewSqlConnFromSession(session)
			nodeModel := configModel.NewAeMcpTaskNodeModel(sessConn)
			chainModel := configModel.NewAeMcpTaskChainModel(sessConn)
			serviceConfigModel := configModel.NewAeMcpExternalServicesConfigModel(sessConn)
			serviceModel := mcp.NewAeMcpServicesModel(sessConn)

			// 查询服务配置
			config, err := serviceConfigModel.FindOneByCondition(ctx, []models.Condition{
				{
					Field: "server_id",
					Value: service.ServerId,
				},
			})
			if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
				return err
			}
			if service.TaskChainId > 0 {
				return nil
			}
			// 创建节点
			var node = configModel.AeMcpTaskNode{
				NodeName:          config.Name + " Node",
				NodeHandle:        "proxy_handle",
				Enabled:           true,
				ExternalServiceId: config.ExternalServiceId,
				Description:       "Proxy " + config.Name + " Node",
			}
			nodeId, err1 := nodeModel.InsertReturningId(ctx, &node)
			if err1 != nil {
				return err1
			}
			// 创建链
			var chain = configModel.AeMcpTaskChain{
				Name:    config.Name + " ProxyChain (" + config.Type + ")",
				Status:  "used",
				NodeIds: pq.Int64Array{9, nodeId, 10},
			}
			chainId, err2 := chainModel.InsertReturningId(ctx, &chain)
			if err2 != nil {
				return err2
			}
			// 更新服务
			service.TaskChainId = chainId
			err = serviceModel.Update(ctx, service)
			if err != nil {
				return err
			}
			// 更新服务配置
			config.CreateStatus = true
			err1 = serviceConfigModel.Update(ctx, config)
			if err1 != nil {
				l.Errorf("更新 config error: %s", err1.Error())
			}
			num++
			return nil
		})
		if err != nil {
			l.Errorf("创建 services failed, err: %s,ServerId:%s", err, service.ServerId)
		}
	}
	l.Infof("处理完成...")
	l.Infof("数量：%d", num)
}
