package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"AgentEarth-Mgr/models"

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

func (l *InnerMcpInfoLogic) InnerMcpInfo(wemcpName string) (resp *types.BaseResp, err error) {
	rawWemcpName := strings.TrimSpace(wemcpName)
	if len(rawWemcpName) == 0 {
		return nil, errors.New("wemcp_name is required")
	}

	cfg, err := l.svcCtx.TaskNodeConfigV2Model.FindOneByCondition(l.ctx, []models.Condition{
		{
			Field:  "wemcp_name",
			Symbol: "=",
			Value:  rawWemcpName,
		},
	})
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, err
	}
	if cfg == nil {
		return nil, errors.New("service config not found for wemcp_name")
	}

	needKey := cfg.AccountRequired == 1
	if !needKey {
		return &types.BaseResp{
			Code:    0,
			Message: "success",
			Data: types.D{
				"need_key": false,
				"envs":     map[string]string{},
			},
		}, nil
	}

	accounts, _, err := l.svcCtx.TaskNodeConfigAccountModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field:  "config_id",
				Symbol: "=",
				Value:  cfg.Id,
			},
			{
				Field:  "status",
				Symbol: "=",
				Value:  "used",
			},
		},
		Pages: models.Pages{
			Page: 1,
			Size: 1,
		},
	}, true)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return &types.BaseResp{
			Code:    1,
			Message: "no used account found",
			Data: types.D{
				"need_key": true,
				"envs":     map[string]string{},
			},
		}, nil
	}
	acc := accounts[0]

	authInfoRaw := strings.TrimSpace(acc.AuthInfo)
	if authInfoRaw == "" {
		return &types.BaseResp{
			Code:    1,
			Message: "auth_info is empty",
			Data: types.D{
				"need_key": true,
				"envs":     map[string]string{},
			},
		}, nil
	}

	authInfo := map[string]interface{}{}
	if unmarshalErr := json.Unmarshal([]byte(authInfoRaw), &authInfo); unmarshalErr != nil {
		return &types.BaseResp{
			Code:    1,
			Message: "auth_info is not valid json",
			Data: types.D{
				"need_key": true,
				"envs":     map[string]string{},
			},
		}, nil
	}

	envs := map[string]string{}
	for k, v := range authInfo {
		if strings.TrimSpace(k) == "" {
			continue
		}
		switch vv := v.(type) {
		case string:
			envs[k] = vv
		default:
			envs[k] = fmt.Sprint(vv)
		}
	}

	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"need_key": true,
			"envs":     envs,
		},
	}

	return resp, nil
}
