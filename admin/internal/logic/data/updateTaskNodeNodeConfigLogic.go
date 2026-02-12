package data

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type UpdateTaskNodeNodeConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateTaskNodeNodeConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateTaskNodeNodeConfigLogic {
	return &UpdateTaskNodeNodeConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateTaskNodeNodeConfigLogic) UpdateTaskNodeNodeConfig(req *types.UpdateTaskNodeNodeConfigReq) (resp *types.BaseResp, err error) {
	if req == nil || strings.TrimSpace(req.ServerId) == "" {
		return &types.BaseResp{
			Code:    -1,
			Message: "server_id不能为空",
		}, nil
	}

	normalized, err := normalizeJsonString(req.NodeConfig)
	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "node_config不是合法JSON: " + err.Error(),
		}, nil
	}

	serverId := strings.TrimSpace(req.ServerId)
	node, err := l.svcCtx.TaskNodeModel.FindOneByServerId(l.ctx, serverId)
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

	node.NodeConfig = normalized
	if err := l.svcCtx.TaskNodeModel.Update(l.ctx, node); err != nil {
		return nil, err
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}, nil
}

func normalizeJsonString(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "{}", nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

