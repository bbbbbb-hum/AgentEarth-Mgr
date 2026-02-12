package mcp

import (
	"AgentEarth-Mgr/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type InnerMcpInfoLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewInnerMcpInfoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InnerMcpInfoLogic {
	return &InnerMcpInfoLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *InnerMcpInfoLogic) InnerMcpInfo(serverId string) (resp *types.BaseResp, err error) {
	if len(serverId) == 0 {
		l.Errorf("[mcpinfo] server_id is required")
		return nil, errors.New("server_id is required")
	}

	config, err := l.svcCtx.TaskNodeConfigModel.FindOneByCondition(l.ctx, []models.Condition{
		{
			Field:  "server_id",
			Symbol: "=",
			Value:  serverId,
		},
	})
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			msg := fmt.Sprintf("config not found for server_id=%s", serverId)
			l.Errorf("[mcpinfo] %s", msg)
			return nil, errors.New(msg)
		}
		l.Errorf("[mcpinfo] server_id=%s find config error: %v", serverId, err)
		return nil, err
	}

	accounts, total, err := l.svcCtx.TaskNodeConfigAccountModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field:  "config_id",
				Symbol: "=",
				Value:  config.Id,
			},
			{
				Field:  "status",
				Symbol: "=",
				Value:  "used",
			},
		},
	}, true)
	if err != nil {
		l.Errorf("[mcpinfo] server_id=%s get account list error: %v", serverId, err)
		return nil, err
	}
	if total == 0 || len(accounts) == 0 {
		msg := fmt.Sprintf("no used account for server_id=%s", serverId)
		l.Errorf("[mcpinfo] %s", msg)
		return nil, errors.New(msg)
	}

	account := accounts[0]
	envs := map[string]interface{}{}
	if len(account.AuthInfo) > 0 {
		if err := json.Unmarshal([]byte(account.AuthInfo), &envs); err != nil {
			l.Errorf("[mcpinfo] server_id=%s auth_info json unmarshal error: %v", serverId, err)
			return nil, err
		}
	}

	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"server_id": config.ServerId,
			"name":      config.Name,
			"envs":      envs,
		},
	}

	return resp, nil
}
