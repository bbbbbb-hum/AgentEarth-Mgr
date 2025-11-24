package svc

import (
	"AgentEarth-Mgr/admin/internal/config"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/external"
	"AgentEarth-Mgr/models/mcp"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                   config.Config
	DB                       sqlx.SqlConn
	McpServiceModel          mcp.AeMcpServicesModel
	ExternalMcpServicesModel external.ExternalMcpServicesModel
	TaskNodeConfigModel      configModel.AeMcpExternalServicesConfigModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewSqlConn("postgres", c.DB.DataSource)
	return &ServiceContext{
		Config:                   c,
		DB:                       db,
		ExternalMcpServicesModel: external.NewExternalMcpServicesModel(db),
		McpServiceModel:          mcp.NewAeMcpServicesModel(db),
		TaskNodeConfigModel:      configModel.NewAeMcpExternalServicesConfigModel(db),
	}
}
