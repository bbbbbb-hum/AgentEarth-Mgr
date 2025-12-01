package data

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"context"
	"encoding/json"
	"strings"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateConfigLogic {
	return &CreateConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateConfigLogic) CreateConfig(req *types.CreateConfigReq) (resp *types.BaseResp, err error) {
	// 获取外部配置列表
	externalMcpServices, total, err := l.svcCtx.ExternalMcpServicesModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			//{
			//	Field:  "valuable",
			//	Symbol: "=",
			//	Value:  1,
			//},
			//{
			//	Field: "server_type",
			//	Value: "httpStream",
			//},
			{
				Field: "test_status",
				Value: req.TestStatus,
			},
		},
	}, true)
	if err != nil {
		return nil, err
	}
	var serviceConfigs []configModel.AeMcpExternalServicesConfig
	if total > 0 {
		var serviceIds []int64
		for _, service := range externalMcpServices {
			projectName := service.ProjectName.String
			if service.ProjectName.Valid == false {
				projectName = createProjectName(service.ServerName)
			}

			var serviceConfig = configModel.AeMcpExternalServicesConfig{
				Name:        service.ServerName,
				MaxInstance: 1,
				LaunchInfo:  "{}",
				ConnectInfo: "{}",
				Description: service.Description,
				ProjectName: projectName,
			}
			switch service.ServerType {
			case "httpStream":
				serviceConfig.Type = "httpStreamable"
				if len(service.ConnectInfo) > 0 {
					var chosenConn flatConnect
					var nc nestedConnect
					if err = json.Unmarshal([]byte(service.ConnectInfo), &nc); err == nil && len(nc.McpServers) > 0 {
						if v, ok := nc.McpServers[service.ServerName]; ok {
							chosenConn = v
						} else {
							for _, v1 := range nc.McpServers {
								chosenConn = v1
								break
							}
						}
					} else {
						var fc flatConnect
						if err = json.Unmarshal([]byte(service.ConnectInfo), &fc); err == nil {
							chosenConn = fc
						} else {
							l.Errorf("failed to parse connect_info for %s: %v", service.ServerName, err)
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
							l.Errorf("failed to marshal target connect_info for %s: %v", service.ServerName, err)
						}
					}
				}
			case "stdio":
				serviceConfig.Type = "stdio"
				// stdio has no connect_info; build launch_info only
				// Parse external launch info (support both nested and flat formats)
				var chosen flatLaunch
				var n nestedLaunch
				if len(service.LaunchInfo) > 0 {
					if err = json.Unmarshal([]byte(service.LaunchInfo), &n); err == nil && len(n.McpServers) > 0 {
						// Prefer entry matching the service name, otherwise pick the first
						if v, ok := n.McpServers[service.ServerName]; ok {
							chosen = v
						} else {
							for _, v1 := range n.McpServers {
								chosen = v1
								break
							}
						}
					} else {
						var f flatLaunch
						if err = json.Unmarshal([]byte(service.LaunchInfo), &f); err == nil {
							chosen = f
						} else {
							l.Errorf("failed to parse launch_info for %s: %v", service.ServerName, err)
						}
					}
				}

				// Build target launch info with defaults
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
					l.Errorf("failed to marshal target launch_info for %s: %v", service.ServerName, err)
					continue
				}
			case "sse":
				serviceConfig.Type = "sse"
				// ensure non-null launch_info for jsonb column
				//serviceConfig.LaunchInfo = "{}"
				// Build connect_info for sse types
				if len(service.ConnectInfo) > 0 {
					var chosenConn flatConnect
					var nc nestedConnect
					if err := json.Unmarshal([]byte(service.ConnectInfo), &nc); err == nil && len(nc.McpServers) > 0 {
						if v, ok := nc.McpServers[service.ServerName]; ok {
							chosenConn = v
						} else {
							for _, v := range nc.McpServers {
								chosenConn = v
								break
							}
						}
					} else {
						var fc flatConnect
						if err := json.Unmarshal([]byte(service.ConnectInfo), &fc); err == nil {
							chosenConn = fc
						} else {
							l.Errorf("failed to parse connect_info for %s: %v", service.ServerName, err)
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
							l.Errorf("failed to marshal target connect_info for %s: %v", service.ServerName, err)
						}
					}
				}
			default:
				l.Errorf("unknown server type: %s", service.ServerType)
				continue
			}
			// external_service_id is generated by DB default (uuid), do not set here

			// Collect for batch insert
			serviceConfigs = append(serviceConfigs, serviceConfig)
			serviceIds = append(serviceIds, service.Id)
		}
		// Batch insert after loop
		if err = l.svcCtx.TaskNodeConfigModel.BatchInsert(l.ctx, serviceConfigs); err != nil {
			l.Errorf("batch insert configs failed: %v", err)
		}
		// 按服务id批量更新service表test_status字段
		if err = l.svcCtx.ExternalMcpServicesModel.BatchUpdateTestStatus(l.ctx, serviceIds, 3); err != nil {
			l.Errorf("batch update test_status failed: %v", err)
		}
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"list":  serviceConfigs,
			"total": len(serviceConfigs),
		},
	}
	return
}

type flatLaunch struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Workdir string            `json:"workdir"`
}
type nestedLaunch struct {
	McpServers map[string]flatLaunch `json:"mcpServers"`
}
type targetLaunch struct {
	Command         string            `json:"command"`
	Args            []string          `json:"args"`
	Workdir         string            `json:"workdir,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	MaxInstance     int               `json:"max_instance"`
	MaxRestarts     int               `json:"max_restarts"`
	LaunchTimeout   int               `json:"launch_timeout"`
	ShutdownTimeout int               `json:"shutdown_timeout"`
	IdleTtl         int               `json:"idle_ttl"`
}

type flatConnect struct {
	URL     string            `json:"url"`
	Type    string            `json:"type,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}
type nestedConnect struct {
	McpServers map[string]flatConnect `json:"mcpServers"`
}
type targetConnect struct {
	URL            string            `json:"url"`
	Headers        map[string]string `json:"headers,omitempty"`
	ConnectTimeout int               `json:"connect_timeout"`
	MaxConnect     int               `json:"max_connect"`
	MaxRetry       int               `json:"max_retry"`
	Interval       int               `json:"interval"`
}

// 创建项目名称
func createProjectName(name string) string {
	// 生成项目名 用 @符号+name 去拼接 把name中的空格去掉，然后空格后的首字母改大写
	projectName := strings.ReplaceAll(name, " ", "")
	projectName = strings.Title(projectName)
	return "@" + projectName
}
