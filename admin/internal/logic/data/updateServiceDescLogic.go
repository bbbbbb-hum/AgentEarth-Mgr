package data

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/mcp"
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type UpdateServiceDescLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateServiceDescLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateServiceDescLogic {
	return &UpdateServiceDescLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateServiceDescLogic) UpdateServiceDesc(req *types.UpdateServiceDescReq) (resp *types.BaseResp, err error) {
	if len(req.ServiceID) > 0 {

	} else {
		//查询所有外部服务配置列表
		configs, total, err1 := l.svcCtx.TaskNodeConfigModel.GetList(l.ctx, models.ListConditions{
			Conditions: []models.Condition{
				{
					Field:  "create_status",
					Symbol: "=",
					Value:  true,
				},
			},
		}, true)
		if err1 != nil {
			err = err1
			return
		}
		if total > 0 {
			go func(configs []*configModel.AeMcpExternalServicesConfig) {
				ctx := context.Background()
				for _, config := range configs {
					err = l.UpdateDesc(ctx, config)
					if err != nil {
						logx.Errorw("UpdateDesc", logx.Field("error", err))
						continue
					}
				}
			}(configs)

		}

	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}
	return
}

func (l *UpdateServiceDescLogic) UpdateDesc(ctx context.Context, taskNodeConfig *configModel.AeMcpExternalServicesConfig) (err error) {
	// 按服务名称获取外部服务表信息
	externalMcpServices, total, err := l.svcCtx.ExternalMcpServicesModel.GetList(ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field:  "server_name",
				Symbol: "=",
				Value:  taskNodeConfig.Name,
			},
		},
	}, true)
	if err != nil {
		return err
	}
	if total > 0 && len(externalMcpServices) > 0 {
		// 更新外部服务配置表的描述与服务表的描述
		err = l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			sessConn := sqlx.NewSqlConnFromSession(session)
			taskNodeConfigModel := configModel.NewAeMcpExternalServicesConfigModel(sessConn)
			mcpServicesModel := mcp.NewAeMcpServicesModel(sessConn)

			taskNodeConfig.Description = externalMcpServices[0].Description
			err = taskNodeConfigModel.Update(ctx, taskNodeConfig)
			if err != nil {
				return err
			}
			mcpServices, _, err := mcpServicesModel.GetList(ctx, models.ListConditions{
				Conditions: []models.Condition{
					{
						Field:  "server_name",
						Symbol: "=",
						Value:  taskNodeConfig.Name,
					},
				},
			}, true)
			if err != nil {
				return err
			}
			if len(mcpServices) > 0 {
				mcpService := mcpServices[0]
				mcpService.Description = externalMcpServices[0].Description
				err = mcpServicesModel.Update(ctx, mcpServices[0])
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
