package svc

import (
	"AgentEarth-Mgr/admin/internal/config"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/external"
	"AgentEarth-Mgr/models/fund"
	"AgentEarth-Mgr/models/mcp"
	"AgentEarth-Mgr/models/users"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                          config.Config
	DB                              sqlx.SqlConn
	McpServiceModel                 mcp.AeMcpServicesModel
	McpServicesInstallModel         mcp.AeMcpServicesInstallModel
	ExternalMcpServicesModel        external.ExternalMcpServicesModel
	ExternalMpcServicesAccountModel external.ExternalMcpServicesAccountModel
	TaskChainModel                  configModel.AeMcpTaskChainModel
	TaskNodeModel                   configModel.AeMcpTaskNodeModel
	TaskNodeConfigModel             configModel.AeMcpExternalServicesConfigModel
	TaskNodeConfigAccountModel      configModel.AeMcpExternalServicesAccountModel
	UserModel               users.AeMcpExternalServicesUserModel
	McpUserModel            users.McpUserModel
	UserRechargeRecordModel fund.AeUserRechargeRecordModel
	UserBalanceDailyModel   fund.AeUserBalanceStatisticDailyModel
	UserConsumptionDailyModel fund.AeUserConsumptionRecordDailyModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewSqlConn("postgres", c.DB.DataSource)
	return &ServiceContext{
		Config:                          c,
		DB:                              db,
		ExternalMcpServicesModel:        external.NewExternalMcpServicesModel(db),
		ExternalMpcServicesAccountModel: external.NewExternalMcpServicesAccountModel(db),
		McpServiceModel:                 mcp.NewAeMcpServicesModel(db),
		McpServicesInstallModel:         mcp.NewAeMcpServicesInstallModel(db),
		TaskChainModel:                  configModel.NewAeMcpTaskChainModel(db),
		TaskNodeModel:                   configModel.NewAeMcpTaskNodeModel(db),
		TaskNodeConfigModel:             configModel.NewAeMcpExternalServicesConfigModel(db),
		TaskNodeConfigAccountModel:      configModel.NewAeMcpExternalServicesAccountModel(db),
		UserModel:               users.NewAeMcpExternalServicesUserModel(db),
		McpUserModel:            users.NewMcpUserModel(db),
		UserRechargeRecordModel: fund.NewAeUserRechargeRecordModel(db),
		UserBalanceDailyModel:   fund.NewAeUserBalanceStatisticDailyModel(db),
		UserConsumptionDailyModel: fund.NewAeUserConsumptionRecordDailyModel(db),
	}
}
