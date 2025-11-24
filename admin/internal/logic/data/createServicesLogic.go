package data

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"fmt"
	"math/rand"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CreateServicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateServicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateServicesLogic {
	return &CreateServicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateServicesLogic) CreateServices() (resp *types.BaseResp, err error) {
	// 查询配置信息
	configs, total, err := l.svcCtx.TaskNodeConfigModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field: "create_status",
				Value: false,
			},
		},
	}, true)
	if err != nil {
		return
	}
	if total > 0 {
		go l.dealServices(configs)
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}

	return
}

func (l *CreateServicesLogic) dealServices(configList []*configModel.AeMcpExternalServicesConfig) {
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
			// 不再更新 external_mcp_services.test_status（撤销）
			// 创建服务
			var service = mcp.AeMcpServices{
				Id:              0,
				ServerId:        createServerId(i),
				ServerName:      config.Name,
				Logo:            "/assets/logo.png",
				ProtocolVersion: "2024-11-05",
				Enabled:         false,
				Tags:            pq.StringArray{"remote", config.Type},
				Description:     config.Description,
				TaskChainId:     chainId,
				CallNum:         0,
				ProjectName:     config.ProjectName,
			}
			if _, err1 = serviceModel.InsertWithoutId(ctx, &service); err1 != nil {
				return err1
			}
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

func createServerId(index int) string {
	// index padded to 5 digits, plus 2-digit random suffix
	rand.Seed(time.Now().UnixNano())
	var randomTwoDigits = rand.Intn(100) // 0-99
	return fmt.Sprintf("%s%05d%02d", "server_", index, randomTwoDigits)
}
