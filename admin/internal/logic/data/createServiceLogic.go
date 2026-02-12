package data

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"errors"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CreateServiceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateServiceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateServiceLogic {
	return &CreateServiceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateServiceLogic) CreateService() (resp *types.BaseResp, err error) {
	// 查询配置信息
	configs, total, err := l.svcCtx.TaskNodeConfigModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field:  "create_status",
				Symbol: "=",
				Value:  false,
			},
		},
	}, true)
	if err != nil {
		return
	}
	if total > 0 {
		go l.dealService(configs)
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}
	return
}

func (l *CreateServiceLogic) dealService(configList []*configModel.AeMcpExternalServicesConfig) {
	var ctx = context.Background()
	var num int64
	for i, config := range configList {
		// one transaction per config
		err := l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			// session-scoped models via constructors bound to the session
			sessConn := sqlx.NewSqlConnFromSession(session)
			nodeModel := configModel.NewAeMcpTaskNodeModel(sessConn)
			chainModel := configModel.NewAeMcpTaskChainModel(sessConn)
			serviceConfigModel := configModel.NewAeMcpExternalServicesConfigModel(sessConn)
			serviceModel := mcp.NewAeMcpServicesModel(sessConn)

			//获取节点
			node, err := nodeModel.FindOneByServerId(ctx, config.ServerId)
			if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
				return err
			}
			var nodeId int64
			if node != nil {
				nodeId = node.Id
			} else {
				// 创建节点
				node = &configModel.AeMcpTaskNode{
					NodeName:    config.Name + " Node",
					NodeHandle:  "proxy_handle",
					Enabled:     true,
					ServerId:    config.ServerId,
					Description: config.Description,
				}
				nodeId, err = nodeModel.InsertReturningId(ctx, node)
				if err != nil {
					return err
				}
			}

			// 获取链
			chains, chainsTotal, err := chainModel.GetList(ctx, models.ListConditions{
				Pages: models.Pages{},
				Conditions: []models.Condition{
					{
						Field:  "node_ids",
						Symbol: "@>",
						Value:  nodeId,
					},
				},
				Sorts: nil,
			}, true)
			if err != nil {
				return err
			}
			var chainId int64
			if chainsTotal > 0 && len(chains) > 0 {
				chainId = chains[0].Id
			} else {
				// 创建链
				var chain = configModel.AeMcpTaskChain{
					Name:    config.Name + " ProxyChain(" + config.Type + ")",
					Status:  "used",
					NodeIds: pq.Int64Array{9, nodeId, 10},
				}
				chainId, err = chainModel.InsertReturningId(ctx, &chain)
				if err != nil {
					return err
				}
			}
			// 创建服务
			var service = mcp.AeMcpServices{
				ServerId:        createServerId(i),
				ServerName:      config.Name,
				Logo:            "/assets/logo.png",
				ProtocolVersion: "2024-11-05",
				Enabled:         true,
				Tags:            pq.StringArray{"remote", config.Type},
				Description:     config.Description,
				TaskChainId:     chainId,
				CallNum:         0,
				ProjectName:     config.ProjectName,
			}
			if _, err = serviceModel.InsertWithoutId(ctx, &service); err != nil {
				return err
			}
			config.CreateStatus = true
			err = serviceConfigModel.Update(ctx, config)
			if err != nil {
				l.Errorf("更新 config error: %s", err.Error())
			}
			num++
			return nil
		})
		if err != nil {
			l.Errorf("创建 services failed, err: %s,configName:%s", err, config.Name)
		}
	}
	l.Infof("处理完成...")
	l.Infof("数量：%d", num)
}
