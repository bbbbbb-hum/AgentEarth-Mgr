package mcp

import (
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"errors"
	"fmt"

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
			serviceModel := mcp.NewAeMcpServicesModel(sessConn)

			var config *configModel.AeMcpExternalServicesConfig
			if service.TaskChainId > 0 {
				return nil
			}
			// 创建节点
			nodeName := service.ServerName + " Node"
			nodeDescription := "Proxy " + service.ServerName + " Node"
			if config != nil {
				nodeName = config.Name + " Node"
				nodeDescription = "Proxy " + config.Name + " Node"
			}
			var node = configModel.AeMcpTaskNode{
				NodeName:    nodeName,
				NodeHandle:  "proxy_handle",
				Enabled:     true,
				Description: nodeDescription,
				ServerId:    service.ServerId,
				NodeConfig:  "{}",
			}
			nodeId, err1 := nodeModel.InsertReturningId(ctx, &node)
			if err1 != nil {
				var pqErr *pq.Error
				if errors.As(err1, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "mcp_task_node_pkey" {
					if resetErr := l.resetTaskNodeSeq(); resetErr != nil {
						return resetErr
					}
					nodeId, err1 = nodeModel.InsertReturningId(ctx, &node)
				}
				if err1 != nil {
					return err1
				}
			}
			// 创建链
			chainName := service.ServerName + " ProxyChain"
			if config != nil {
				chainName = config.Name + " ProxyChain (" + config.Type + ")"
			}
			var chain = configModel.AeMcpTaskChain{
				Name:    chainName,
				Status:  "used",
				NodeIds: pq.Int64Array{9, nodeId, 10},
			}
			chainId, err2 := chainModel.InsertReturningId(ctx, &chain)
			if err2 != nil {
				var pqErr *pq.Error
				if errors.As(err2, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "mcp_task_chain_pkey" {
					if resetErr := l.resetTaskChainSeq(); resetErr != nil {
						return resetErr
					}
					chainId, err2 = chainModel.InsertReturningId(ctx, &chain)
				}
				if err2 != nil {
					return err2
				}
			}
			// 更新服务
			service.TaskChainId = chainId
			if err := serviceModel.Update(ctx, service); err != nil {
				return err
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
	return l.resetSeq("public.ae_mcp_task_node", "id")
}

func (l *ServiceTaskCreateLogic) resetTaskChainSeq() error {
	return l.resetSeq("public.ae_mcp_task_chain", "id")
}

func (l *ServiceTaskCreateLogic) resetSeq(table string, column string) error {
	var seq string
	seqQuery := `select pg_get_serial_sequence($1, $2)`
	err := l.svcCtx.DB.QueryRowCtx(l.ctx, &seq, seqQuery, table, column)
	if err != nil {
		return err
	}
	if seq == "" {
		return errors.New("序列不存在")
	}
	setvalQuery := fmt.Sprintf(`select setval($1, (select coalesce(max(%s), 0) + 1 from %s), false)`, column, table)
	_, err = l.svcCtx.DB.ExecCtx(l.ctx, setvalQuery, seq)
	return err
}
