package svc

import (
	"AgentEarth-Mgr/admin/internal/config"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/external"
	"AgentEarth-Mgr/models/mcp"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                          config.Config
	DB                              sqlx.SqlConn
	ProdDB                          sqlx.SqlConn
	McpServiceModel                 mcp.AeMcpServicesModel
	McpServicesInstallModel         mcp.AeMcpServicesInstallModel
	ExternalMcpServicesModel        external.ExternalMcpServicesModel
	ProdExternalMcpServicesModel    external.ExternalMcpServicesModel
	ExternalMpcServicesAccountModel external.ExternalMcpServicesAccountModel
	ProdExternalMpcServicesAccountModel external.ExternalMcpServicesAccountModel
	TaskChainModel                  configModel.AeMcpTaskChainModel
	TaskNodeModel                   configModel.AeMcpTaskNodeModel
	TaskNodeConfigModel             configModel.AeMcpExternalServicesConfigModel
	TaskNodeConfigAccountModel      configModel.AeMcpExternalServicesAccountModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewSqlConn("postgres", c.DB.DataSource)
	var prodDb sqlx.SqlConn
	var prodExternalMcpServicesModel external.ExternalMcpServicesModel
	var prodExternalMpcServicesAccountModel external.ExternalMcpServicesAccountModel
	if len(c.ProdDB.DataSource) > 0 {
		prodDb = sqlx.NewSqlConn("postgres", c.ProdDB.DataSource)
		prodExternalMcpServicesModel = external.NewExternalMcpServicesModel(prodDb)
		prodExternalMpcServicesAccountModel = external.NewExternalMcpServicesAccountModel(prodDb)
	}
	return &ServiceContext{
		Config:                          c,
		DB:                              db,
		ProdDB:                          prodDb,
		ExternalMcpServicesModel:        external.NewExternalMcpServicesModel(db),
		ProdExternalMcpServicesModel:    prodExternalMcpServicesModel,
		ExternalMpcServicesAccountModel: external.NewExternalMcpServicesAccountModel(db),
		ProdExternalMpcServicesAccountModel: prodExternalMpcServicesAccountModel,
		McpServiceModel:                 mcp.NewAeMcpServicesModel(db),
		McpServicesInstallModel:         mcp.NewAeMcpServicesInstallModel(db),
		TaskChainModel:                  configModel.NewAeMcpTaskChainModel(db),
		TaskNodeModel:                   configModel.NewAeMcpTaskNodeModel(db),
		TaskNodeConfigModel:             configModel.NewAeMcpExternalServicesConfigModel(db),
		TaskNodeConfigAccountModel:      configModel.NewAeMcpExternalServicesAccountModel(db),
	}
}
