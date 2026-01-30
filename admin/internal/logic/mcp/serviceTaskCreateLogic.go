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
				Value:  req.Ids,
			},
		},
	}, true)
	if err != nil {
		return
	}
	if total > 0 {
		if err := l.deal(serviceList); err != nil {
			return nil, err
		}
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

func (l *ServiceTaskCreateLogic) deal(services []*mcp.AeMcpServices) error {
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
			if config == nil {
				return errors.New("服务配置不存在")
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
				var pqErr *pq.Error
				if errors.As(err1, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "mcp_task_node_pkey" {
					if resetErr := l.resetTaskNodeSeq(); resetErr != nil {
						return err1
					}
					nodeId, err1 = nodeModel.InsertReturningId(ctx, &node)
				}
				if err1 != nil {
					return err1
				}
			}
			// 创建链
			var chain = configModel.AeMcpTaskChain{
				Name:    config.Name + " ProxyChain (" + config.Type + ")",
				Status:  "used",
				NodeIds: pq.Int64Array{9, nodeId, 10},
			}
			chainId, err2 := chainModel.InsertReturningId(ctx, &chain)
			if err2 != nil {
				var pqErr *pq.Error
				if errors.As(err2, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "mcp_task_chain_pkey" {
					if resetErr := l.resetTaskChainSeq(); resetErr != nil {
						return err2
					}
					chainId, err2 = chainModel.InsertReturningId(ctx, &chain)
				}
				if err2 != nil {
					return err2
				}
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
			return err
		}
	}
	l.Infof("处理完成...")
	l.Infof("数量：%d", num)
	return nil
}

func (l *ServiceTaskCreateLogic) resetTaskNodeSeq() error {
	query := `select setval('"public"."ae_mcp_task_node_id_seq"', (select coalesce(max(id), 0) + 1 from "public"."ae_mcp_task_node"), false)`
	_, err := l.svcCtx.DB.ExecCtx(l.ctx, query)
	return err
}

func (l *ServiceTaskCreateLogic) resetTaskChainSeq() error {
	query := `select setval('"public"."ae_mcp_task_chain_id_seq"', (select coalesce(max(id), 0) + 1 from "public"."ae_mcp_task_chain"), false)`
	_, err := l.svcCtx.DB.ExecCtx(l.ctx, query)
	return err
}
