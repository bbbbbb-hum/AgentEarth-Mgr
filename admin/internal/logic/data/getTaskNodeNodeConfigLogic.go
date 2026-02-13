package data

import (
	"context"
	"errors"
	"strings"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type GetTaskNodeNodeConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTaskNodeNodeConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTaskNodeNodeConfigLogic {
	return &GetTaskNodeNodeConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTaskNodeNodeConfigLogic) GetTaskNodeNodeConfig(req *types.GetTaskNodeNodeConfigReq) (resp *types.BaseResp, err error) {
	if req == nil || len(strings.TrimSpace(req.ServerId)) == 0 {
		return &types.BaseResp{
			Code:    -1,
			Message: "server_id不能为空",
		}, nil
	}

	node, err := l.svcCtx.TaskNodeModel.FindOneByServerId(l.ctx, strings.TrimSpace(req.ServerId))
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, err
	}
	if node == nil {
		return &types.BaseResp{
			Code:    1,
			Message: "未找到节点",
			Data:    types.D{},
		}, nil
	}

	nodeConfig := strings.TrimSpace(node.NodeConfig)
	if len(nodeConfig) == 0 {
		nodeConfig = "{}"
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"server_id":   node.ServerId,
			"node_id":     node.Id,
			"node_config": nodeConfig,
		},
	}, nil
}

