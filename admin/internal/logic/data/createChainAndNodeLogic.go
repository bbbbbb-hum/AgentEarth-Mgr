package data

import (
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	if req == nil || len(req.ServerId) == 0 {
		return &types.BaseResp{
			Code:    -1,
			Message: "server_id不能为空",
		}, nil
	}

	if err := l.deal(req); err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: err.Error(),
		}, nil
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}
	return
}

func (l *CreateChainAndNodeLogic) deal(req *types.CreateChainAndNodeReq) error {
	var ctx = context.Background()
	return l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		sessConn := sqlx.NewSqlConnFromSession(session)
		nodeModel := configModel.NewAeMcpTaskNodeModel(sessConn)
		chainModel := configModel.NewAeMcpTaskChainModel(sessConn)
		serviceModel := mcp.NewAeMcpServicesModel(sessConn)

		mcpService, err := serviceModel.FindOne(ctx, req.ServerId)
		if err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				return fmt.Errorf("服务不存在: %s", req.ServerId)
			}
			return err
		}

		nodeName := req.NodeName
		if len(nodeName) == 0 {
			nodeName = mcpService.ServerName + " Node"
		}
		nodeHandle := req.NodeHandle
		if len(nodeHandle) == 0 {
			nodeHandle = "proxy_handle"
		}

		nodeConfig := strings.TrimSpace(req.NodeConfig)
		if len(nodeConfig) == 0 {
			nodeConfig = "{}"
		} else {
			var tmp interface{}
			if err := json.Unmarshal([]byte(nodeConfig), &tmp); err != nil {
				return fmt.Errorf("node_config不是合法JSON: %s", err.Error())
			}
		}

		node := configModel.AeMcpTaskNode{
			NodeName:    nodeName,
			NodeHandle:  nodeHandle,
			Enabled:     true,
			Description: mcpService.ServerName + " Node",
			ServerId:    mcpService.ServerId,
			NodeConfig:  nodeConfig,
		}
		nodeId, err := nodeModel.InsertReturningId(ctx, &node)
		if err != nil {
			return err
		}

		chainName := req.ChainName
		if len(chainName) == 0 {
			chainName = mcpService.ServerName + " Chain"
		}
		chain := configModel.AeMcpTaskChain{
			Name:    chainName,
			Status:  "used",
			NodeIds: pq.Int64Array{9, nodeId, 10},
		}
		chainId, err := chainModel.InsertReturningId(ctx, &chain)
		if err != nil {
			return err
		}

		mcpService.TaskChainId = chainId
		return serviceModel.Update(ctx, mcpService)
	})
}
