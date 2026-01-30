package data

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/external"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CreateServiceAndConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateServiceAndConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateServiceAndConfigLogic {
	return &CreateServiceAndConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateServiceAndConfigLogic) CreateServiceAndConfig(req *types.CreateServiceAndConfigReq) (resp *types.BaseResp, err error) {
	// 获取外部配置列表
	externalMcpServices, total, err := l.svcCtx.ExternalMcpServicesModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field: "test_status",
				Value: req.TestStatus,
			},
		},
	}, true)
	if err != nil {
		return nil, err
	}
	if total > 0 {
		go l.deal(externalMcpServices)
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"total": total,
		},
	}
	return
}

func (l *CreateServiceAndConfigLogic) deal(externalMcpServices []*external.ExternalMcpServices) {
	ctx := context.Background()
	var insertNum, updateNum int64
	for _, ems := range externalMcpServices {
		err := l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			sessConn := sqlx.NewSqlConnFromSession(session)
			serviceConfigModel := configModel.NewAeMcpExternalServicesConfigModel(sessConn)
			serviceModel := mcp.NewAeMcpServicesModel(sessConn)
			externalMcpServiceModel := external.NewExternalMcpServicesModel(sessConn)
			projectName := ems.ProjectName.String
			if ems.ProjectName.Valid == false {
				projectName = CreateProjectName(ems.ServerName)
			}
			// 判断服务配置是否存在，存在则更新
			serviceConfig, err1 := serviceConfigModel.FindOneByCondition(ctx, []models.Condition{
				{
					Field: "name",
					Value: ems.ServerName,
				},
			})
			if err1 != nil && !errors.Is(err1, sqlx.ErrNotFound) {
				return err1
			}
			if serviceConfig != nil {
				serviceConfig.Name = ems.ServerName
				serviceConfig.Description = ems.Description
				serviceConfig.ProjectName = projectName
			} else {
				// 创建服务配置
				serviceConfig = &configModel.AeMcpExternalServicesConfig{
					Name:        ems.ServerName,
					MaxInstance: 1,
					LaunchInfo:  "{}",
					ConnectInfo: "{}",
					Description: ems.Description,
					ProjectName: projectName,
				}
			}
			switch ems.ServerType {
			case "httpStream":
				serviceConfig.Type = "httpStreamable"
				if len(ems.ConnectInfo) > 0 {
					var chosenConn flatConnect
					var nc nestedConnect
					if err := json.Unmarshal([]byte(ems.ConnectInfo), &nc); err == nil && len(nc.McpServers) > 0 {
						if v, ok := nc.McpServers[ems.ServerName]; ok {
							chosenConn = v
						} else {
							for _, v1 := range nc.McpServers {
								chosenConn = v1
								break
							}
						}
					} else {
						var fc flatConnect
						if err := json.Unmarshal([]byte(ems.ConnectInfo), &fc); err == nil {
							chosenConn = fc
						} else {
							l.Errorf("failed to parse connect_info for %s: %v", ems.ServerName, err)
						}
					}
					if len(chosenConn.URL) > 0 {
						tc := targetConnect{
							URL:            chosenConn.URL,
							Headers:        chosenConn.Headers,
							ConnectTimeout: 3000,
							MaxConnect:     10,
							MaxRetry:       3,
							Interval:       1000,
						}
						if b, err := json.Marshal(tc); err == nil {
							serviceConfig.ConnectInfo = string(b)
						} else {
							l.Errorf("failed to marshal target connect_info for %s: %v", ems.ServerName, err)
						}
					}
				}
			case "stdio":
				serviceConfig.Type = "stdio"
				var chosen flatLaunch
				var n nestedLaunch
				if len(ems.LaunchInfo) > 0 {
					if err := json.Unmarshal([]byte(ems.LaunchInfo), &n); err == nil && len(n.McpServers) > 0 {
						if v, ok := n.McpServers[ems.ServerName]; ok {
							chosen = v
						} else {
							for _, v1 := range n.McpServers {
								chosen = v1
								break
							}
						}
					} else {
						var f flatLaunch
						if err = json.Unmarshal([]byte(ems.LaunchInfo), &f); err == nil {
							chosen = f
						} else {
							l.Errorf("failed to parse launch_info for %s: %v", ems.ServerName, err)
						}
					}
				}
				tl := targetLaunch{
					Command:         chosen.Command,
					Args:            chosen.Args,
					Workdir:         chosen.Workdir,
					Env:             chosen.Env,
					MaxInstance:     1,
					MaxRestarts:     3,
					LaunchTimeout:   100000,
					ShutdownTimeout: 100000,
					IdleTtl:         100000000,
				}
				if b, err := json.Marshal(tl); err == nil {
					serviceConfig.LaunchInfo = string(b)
				} else {
					l.Errorf("failed to marshal target launch_info for %s: %v", ems.ServerName, err)
					return err
				}
			case "sse":
				serviceConfig.Type = "sse"
				if len(ems.ConnectInfo) > 0 {
					var chosenConn flatConnect
					var nc nestedConnect
					if err := json.Unmarshal([]byte(ems.ConnectInfo), &nc); err == nil && len(nc.McpServers) > 0 {
						if v, ok := nc.McpServers[ems.ServerName]; ok {
							chosenConn = v
						} else {
							for _, v := range nc.McpServers {
								chosenConn = v
								break
							}
						}
					} else {
						var fc flatConnect
						if err := json.Unmarshal([]byte(ems.ConnectInfo), &fc); err == nil {
							chosenConn = fc
						} else {
							l.Errorf("failed to parse connect_info for %s: %v", ems.ServerName, err)
						}
					}
					if len(chosenConn.URL) > 0 {
						tc := targetConnect{
							URL:            chosenConn.URL,
							Headers:        chosenConn.Headers,
							ConnectTimeout: 3000,
							MaxConnect:     10,
							MaxRetry:       3,
							Interval:       1000,
						}
						if b, err := json.Marshal(tc); err == nil {
							serviceConfig.ConnectInfo = string(b)
						} else {
							l.Errorf("failed to marshal target connect_info for %s: %v", ems.ServerName, err)
						}
					}
				}
			default:
				err := fmt.Errorf("unknown server type: %s", ems.ServerType)
				return err
			}
			var err error
			if serviceConfig.Id > 0 {
				err = serviceConfigModel.Update(ctx, serviceConfig)
			} else {
				_, err = serviceConfigModel.Insert(ctx, serviceConfig)
			}
			if err != nil {
				l.Errorf("insert config failed: %v", err)
				return err
			}

			// 判断服务是否存在，存在则更新
			mcpService, err2 := serviceModel.FindOneByCondition(ctx, []models.Condition{
				{
					Field: "server_name",
					Value: ems.ServerName,
				},
			})
			if err2 != nil && !errors.Is(err2, sqlx.ErrNotFound) {
				return err2
			}
			if mcpService != nil {
				mcpService.ServerName = ems.ServerName
				mcpService.Description = ems.Description
				mcpService.ProjectName = projectName
				mcpService.Logo = ems.Image.String
			} else {
				id, err := NextServiceId(ctx, sessConn)
				if err != nil {
					return err
				}
				mcpService = &mcp.AeMcpServices{
					Id:              id,
					ServerId:        FormatServerId(id),
					ServerName:      ems.ServerName,
					Logo:            ems.Image.String,
					ProtocolVersion: "2024-11-05",
					Enabled:         false,
					Tags:            pq.StringArray{},
					Description:     ems.Description,
					TaskChainId:     0,
					CallNum:         0,
					ProjectName:     projectName,
				}
			}

			// 转换标签
			var tags = strings.Split(ems.Category.String, ",")
			var validTags []string
			validTags = append(validTags, "All")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					validTags = append(validTags, tag)
				}
			}

			mcpService.Tags = pq.StringArray(validTags)
			// 判断是否需要提前安装
			if ems.CloneRepository.Valid && ems.CloneRepository.String != "" {
				mcpService.IsInstall = true
			}
			//判断更新还是插入
			if mcpService.Id > 0 {
				err2 = serviceModel.Update(ctx, mcpService)
				if err2 != nil {
					l.Errorf("update mcp service failed: %v", err2)
					return err2
				}
				updateNum++
			} else {
				_, err2 = serviceModel.InsertWithId(ctx, mcpService)
				if err2 != nil {
					l.Errorf("insert mcp service failed: %v", err2)
					return err2
				}
				insertNum++
			}

			// 更新外部服务表
			ems.TestStatus = 9
			err = externalMcpServiceModel.Update(ctx, ems)
			if err != nil {
				return err
			}
			return err
		})
		if err != nil {
			l.Errorf("创建 services failed, err: %s,ServiceName:%s", err, ems.ServerName)
		}
	}
	l.Infof("处理完成...")
	l.Infof("新增数量：%d", insertNum)
	l.Infof("更新数量：%d", updateNum)
}
