package mcp

import (
	dataLogic "AgentEarth-Mgr/admin/internal/logic/data"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"strings"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceCreateLogic {
	return &ServiceCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceCreateLogic) ServiceCreate(req *types.ServiceCreateReq) (resp *types.BaseResp, err error) {
	if req == nil || len(strings.TrimSpace(req.ServerName)) == 0 {
		return &types.BaseResp{Code: -1, Message: "服务名称不能为空"}, nil
	}

	projectName := strings.TrimSpace(req.ProjectName)
	if len(projectName) == 0 {
		projectName = dataLogic.CreateProjectName(req.ServerName)
	}
	logo := strings.TrimSpace(req.Logo)
	if len(logo) == 0 {
		logo = "/assets/logo.png"
	}
	protocolVersion := strings.TrimSpace(req.ProtocolVersion)
	if len(protocolVersion) == 0 {
		protocolVersion = "2024-11-05"
	}

	var tags []string
	for _, t := range req.Tags {
		if tt := strings.TrimSpace(t); len(tt) > 0 {
			tags = append(tags, tt)
		}
	}

	returnResp := &types.BaseResp{Code: 0, Message: "success", Data: types.D{}}
	ctx := l.ctx
	err = l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		sessConn := sqlx.NewSqlConnFromSession(session)
		serviceModel := mcp.NewAeMcpServicesModel(sessConn)

		id, err := dataLogic.NextServiceId(ctx, sessConn)
		if err != nil {
			return err
		}
		serverId := dataLogic.FormatServerId(id)

		service := &mcp.AeMcpServices{
			Id:              id,
			ServerId:        serverId,
			ServerName:      strings.TrimSpace(req.ServerName),
			Logo:            logo,
			ProtocolVersion: protocolVersion,
			Enabled:         false,
			Tags:            pq.StringArray(tags),
			Description:     strings.TrimSpace(req.Description),
			TaskChainId:     0,
			CallNum:         0,
			ProjectName:     projectName,
			IsInstall:       false,
			XlcreditPrice:   0,
		}

		if _, err := serviceModel.InsertWithId(ctx, service); err != nil {
			return err
		}

		returnResp.Data = types.D{
			"id":        id,
			"server_id": serverId,
		}
		return nil
	})
	if err != nil {
		return &types.BaseResp{Code: -1, Message: err.Error()}, nil
	}
	return returnResp, nil
}
