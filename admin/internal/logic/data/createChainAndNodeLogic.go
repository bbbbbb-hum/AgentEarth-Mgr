package data

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"errors"
	"fmt"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CreateChainAndNodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateChainAndNodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateChainAndNodeLogic {
	return &CreateChainAndNodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateChainAndNodeLogic) CreateChainAndNode(req *types.CreateChainAndNodeReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	var conditions []models.Condition
	if len(req.Ids) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "id",
			Symbol: "in",
			Value:  req.Ids,
		})
	} else {
		conditions = append(conditions, models.Condition{
			Field: "create_status",
			Value: false,
		})
	}
	configs, total, err := l.svcCtx.TaskNodeConfigModel.GetList(l.ctx, models.ListConditions{
		Conditions: conditions,
	}, true)
	if err != nil {
		return
	}
	if total > 0 {
		go l.deal(configs)
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}
	return
}

func (l *CreateChainAndNodeLogic) deal(configList []*configModel.AeMcpExternalServicesConfig) {
	var ctx = context.Background()
	var num int64
	for _, config := range configList {
		// one transaction per config
		err := l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			// session-scoped models via constructors bound to the session
			sessConn := sqlx.NewSqlConnFromSession(session)
			nodeModel := configModel.NewAeMcpTaskNodeModel(sessConn)
			chainModel := configModel.NewAeMcpTaskChainModel(sessConn)
			serviceConfigModel := configModel.NewAeMcpExternalServicesConfigModel(sessConn)
			serviceModel := mcp.NewAeMcpServicesModel(sessConn)

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
			mcpService, err2 := serviceModel.FindOneByCondition(ctx, []models.Condition{
				{
					Field: "server_name",
					Value: config.Name,
				},
			})
			if err2 != nil && !errors.Is(err2, sqlx.ErrNotFound) {
				return err2
			}
			if mcpService == nil {
				return fmt.Errorf("服务不存在,%v", config.Name)
			}
			mcpService.TaskChainId = chainId
			err := serviceModel.Update(ctx, mcpService)
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
			l.Errorf("创建 services failed, err: %s,configName:%s", err, config.Name)
		}
	}
	l.Infof("处理完成...")
	l.Infof("数量：%d", num)
}
