package mcp

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

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
	rawServerId := strings.TrimSpace(serverId)
	if len(rawServerId) == 0 {
		return nil, errors.New("server_id is required")
	}

	var config *configModel.AeMcpExternalServicesConfig
	c, err := l.svcCtx.TaskNodeConfigModel.FindOneByCondition(l.ctx, []models.Condition{
		{
			Field:  "server_id",
			Symbol: "=",
			Value:  rawServerId,
		},
	})
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, err
	}
	if c != nil {
		config = c
	}
	if config == nil && strings.HasPrefix(rawServerId, "server_") {
		idPart := strings.TrimPrefix(rawServerId, "server_")
		if idPart != "" {
			if cfgId, parseErr := strconv.ParseInt(idPart, 10, 64); parseErr == nil {
				c, err := l.svcCtx.TaskNodeConfigModel.FindOne(l.ctx, cfgId)
				if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
					return nil, err
				}
				if c != nil {
					config = c
					if strings.TrimSpace(config.ServerId) == "" || strings.TrimSpace(config.ServerId) != rawServerId {
						config.ServerId = rawServerId
						_ = l.svcCtx.TaskNodeConfigModel.Update(l.ctx, config)
					}
				}
			}
		}
	}
	if config == nil {
		return nil, errors.New("config not found for server_id")
	}
	if !config.TestStatus.Valid || config.TestStatus.Int64 != 1 {
		return nil, errors.New("config test_status not passed")
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
		return nil, err
	}
	if total == 0 || len(accounts) == 0 {
		return nil, errors.New("no used account found for server_id")
	}

	var account *configModel.AeMcpExternalServicesAccount
	for i := range accounts {
		status := strings.ToLower(strings.TrimSpace(accounts[i].Status))
		if status == "used" {
			account = accounts[i]
			break
		}
	}
	if account == nil {
		return nil, errors.New("no used account found for server_id")
	}
	envs := map[string]interface{}{}
	if len(account.AuthInfo) > 0 {
		if err := json.Unmarshal([]byte(account.AuthInfo), &envs); err != nil {
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
