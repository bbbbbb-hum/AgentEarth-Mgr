package data

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"errors"
	"strings"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CreateServiceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateServiceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateServiceLogic {
	return &CreateServiceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateServiceLogic) CreateService(req *types.CreateServiceReq) (resp *types.BaseResp, err error) {
	// 查询配置信息
	var conditions []models.Condition
	desiredEnabled := true
	if req != nil && req.Enabled != nil {
		desiredEnabled = *req.Enabled
	}
	// 如果显式传入了 Ids，则不限制 create_status，允许强制创建/更新
	if req != nil && len(req.Ids) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "id",
			Symbol: "IN",
			Value:  req.Ids,
		})
	} else {
		// 否则只处理未创建的
		conditions = append(conditions, models.Condition{
			Field:  "create_status",
			Symbol: "=",
			Value:  false,
		})
	}

	configs, total, err := l.svcCtx.TaskNodeConfigModel.GetList(l.ctx, models.ListConditions{
		Conditions: conditions,
	}, true)
	if err != nil {
		return
	}
	if total > 0 {
		if err := l.dealService(configs, desiredEnabled); err != nil {
			return nil, err
		}
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data:    types.D{},
	}
	return
}

func (l *CreateServiceLogic) dealService(configList []*configModel.AeMcpExternalServicesConfig, desiredEnabled bool) error {
	var ctx = context.Background()
	var num int64
	for _, config := range configList {
		// one transaction per config
		err := l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			// session-scoped models via constructors bound to the session
			sessConn := sqlx.NewSqlConnFromSession(session)
			serviceConfigModel := configModel.NewAeMcpExternalServicesConfigModel(sessConn)
			serviceModel := mcp.NewAeMcpServicesModel(sessConn)

			// 检查服务是否存在
			var service *mcp.AeMcpServices

			// 确保 ProjectName 有值
			if config.ProjectName == "" {
				config.ProjectName = CreateProjectName(config.Name)
			}

			// 优先使用 ServerId 查找
			if config.ServerId != "" {
				s, err := serviceModel.FindOne(ctx, config.ServerId)
				if err == nil {
					service = s
				} else if !errors.Is(err, sqlx.ErrNotFound) {
					return err
				}
			}
			// 如果没找到，尝试用名称查找
			if service == nil {
				s, err := serviceModel.FindOneByCondition(ctx, []models.Condition{
					{Field: "server_name", Symbol: "=", Value: config.Name},
				})
				if err == nil {
					service = s
				} else if !errors.Is(err, sqlx.ErrNotFound) {
					return err
				}
			}

			isInstall := config.InstallInfo.Valid && strings.TrimSpace(config.InstallInfo.String) != ""

			if service != nil {
				// 更新现有服务
				service.Logo = "/assets/logo.png"
				service.ProtocolVersion = "2024-11-05"
				service.Enabled = desiredEnabled
				service.Tags = pq.StringArray{"remote", config.Type}
				service.Description = config.Description
				// 不修改 TaskChainId
				service.ProjectName = config.ProjectName
				service.IsInstall = isInstall
				// 确保 ServerId 一致
				if config.ServerId == "" {
					config.ServerId = service.ServerId
				}
				if err := serviceModel.Update(ctx, service); err != nil {
					return err
				}
			} else {
				// 创建新服务
				serverId := config.ServerId
				id, err := NextServiceId(ctx, sessConn)
				if err != nil {
					return err
				}
				serviceId := id
				if serverId == "" {
					serverId = FormatServerId(id)
				}
				service = &mcp.AeMcpServices{
					Id:              serviceId,
					ServerId:        serverId,
					ServerName:      config.Name,
					Logo:            "/assets/logo.png",
					ProtocolVersion: "2024-11-05",
					Enabled:         desiredEnabled,
					Tags:            pq.StringArray{"remote", config.Type},
					Description:     config.Description,
					TaskChainId:     0, // 初始不绑定任务链
					CallNum:         0,
					ProjectName:     config.ProjectName,
					IsInstall:       isInstall,
				}
				if _, err := serviceModel.InsertWithId(ctx, service); err != nil {
					var pqErr *pq.Error
					if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "mcp_server_pkey" {
						if resetErr := l.resetServiceSeq(); resetErr != nil {
							l.Errorf("Reset service seq failed for %s: %v", config.Name, resetErr)
							return err
						}
						if _, retryErr := serviceModel.InsertWithId(ctx, service); retryErr == nil {
							l.Infof("Service inserted successfully after seq reset: %s (ServerId: %s)", config.Name, serverId)
						} else {
							l.Errorf("Insert service failed after seq reset for %s: %v", config.Name, retryErr)
							return retryErr
						}
					} else {
						l.Errorf("Insert service failed for %s: %v", config.Name, err)
						return err
					}
				}
				l.Infof("Service inserted successfully: %s (ServerId: %s)", config.Name, serverId)
				config.ServerId = service.ServerId
			}

			config.CreateStatus = true
			if err := serviceConfigModel.Update(ctx, config); err != nil {
				l.Errorf("更新 config error: %s", err.Error())
				// 关键：如果更新 config 失败，返回 error 以回滚整个事务
				return err
			}
			l.Infof("Config updated successfully for: %s", config.Name)
			num++
			return nil
		})
		if err != nil {
			l.Errorf("创建 services failed, err: %s,configName:%s", err, config.Name)
			return err
		}
	}
	l.Infof("处理完成...")
	l.Infof("数量：%d", num)
	return nil
}

func (l *CreateServiceLogic) resetServiceSeq() error {
	query := `select setval('"public"."ae_mcp_server_id_seq"', (select coalesce(max(id), 0) + 1 from "public"."ae_mcp_services"), false)`
	_, err := l.svcCtx.DB.ExecCtx(l.ctx, query)
	return err
}
