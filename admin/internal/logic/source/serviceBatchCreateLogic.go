package source

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/external"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceBatchCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceBatchCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceBatchCreateLogic {
	return &ServiceBatchCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceBatchCreateLogic) ServiceBatchCreate(req *types.ServiceBatchCreateReq) (resp *types.BaseResp, err error) {
	// 获取外部配置列表
	externalMcpServices, total, err := l.svcCtx.ExternalMcpServicesModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field:  "id",
				Symbol: "in",
				Value:  req.Ids,
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

func (l *ServiceBatchCreateLogic) deal(externalMcpServices []*external.ExternalMcpServices) {
	ctx := context.Background()
	var insertNum, updateNum int64
	for i, ems := range externalMcpServices {
		err := l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			sessConn := sqlx.NewSqlConnFromSession(session)
			serviceConfigModel := configModel.NewAeMcpExternalServicesConfigModel(sessConn)
			serviceModel := mcp.NewAeMcpServicesModel(sessConn)
			externalMcpServiceModel := external.NewExternalMcpServicesModel(sessConn)
			serviceInstallModel := mcp.NewAeMcpServicesInstallModel(sessConn)

			var installCmd, repositoryUrl, repositoryName string
			projectName := ems.ProjectName.String
			if ems.ProjectName.Valid == false {
				projectName = createProjectName(ems.ServerName)
			}

			// ========================================== 处理MCP服务 ==================================================================
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
				// 创建服务，生成唯一的 ServerId
				var serverId string
				maxRetries := 10
				for retry := 0; retry < maxRetries; retry++ {
					serverId = createServerId(i + retry*1000) // 增加偏移量避免重复
					// 检查 ServerId 是否已存在
					_, err := serviceModel.FindOne(ctx, serverId)
					if err != nil {
						if errors.Is(err, sqlx.ErrNotFound) {
							// ServerId 不存在，可以使用
							break
						}
						// 其他错误，返回
						return err
					}
					// ServerId 已存在，继续重试
				}

				mcpService = &mcp.AeMcpServices{
					Id:              0,
					ServerId:        serverId,
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
			if ems.Install.Valid && ems.Install.String != "" {
				mcpService.IsInstall = true
				//获取第一层安装命令
				installCmd = ems.Install.String
				// 从 git clone 命令中提取项目目录名称
			}
			//判断是否需要下载源码
			if ems.CloneRepository.Valid && ems.CloneRepository.String != "" {
				repositoryUrl = ems.CloneRepository.String
				repositoryName = extractProjectDirName(repositoryUrl)
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
				_, err2 = serviceModel.InsertWithoutId(ctx, mcpService)
				if err2 != nil {
					l.Errorf("insert mcp service failed: %v", err2)
					return err2
				}
				insertNum++
			}
			var err error
			// ========================================== 创建/更新MCP安装命令 ============================================================
			// 不需要安装的服务就不创建安装命令了
			//if len(installCmd) > 0 || len(repository) > 0 || len(preInstallCmd) > 0 {
			serviceInstall, err3 := serviceInstallModel.FindOneByCondition(ctx, []models.Condition{
				{
					Field: "server_id",
					Value: mcpService.ServerId,
				},
			})
			if err3 != nil && !errors.Is(err3, sqlx.ErrNotFound) {
				return err3
			}
			if serviceInstall == nil {
				serviceInstall = &mcp.AeMcpServicesInstall{
					ServerId:   mcpService.ServerId,
					CreateTime: time.Now(),
				}
			}

			if len(installCmd) > 0 {
				serviceInstall.InstallCmd = installCmd
			}
			if len(repositoryUrl) > 0 {
				serviceInstall.Repository = repositoryUrl
			}
			// ========================================== 处理服务配置 ==================================================================
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
				serviceConfig.ServerId = mcpService.ServerId
			} else {
				// 创建服务配置
				serviceConfig = &configModel.AeMcpExternalServicesConfig{
					Name:        ems.ServerName,
					MaxInstance: 1,
					LaunchInfo:  "{}",
					ConnectInfo: "{}",
					Description: ems.Description,
					ProjectName: projectName,
					ServerId:    mcpService.ServerId,
				}
			}
			switch ems.ServerType {
			case "httpStream":
				serviceConfig.Type = "httpStreamable"
				if len(ems.ConnectInfo) > 0 {
					var chosenConn types.FlatConnect
					var nc types.NestedConnect
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
						var fc types.FlatConnect
						if err := json.Unmarshal([]byte(ems.ConnectInfo), &fc); err == nil {
							chosenConn = fc
						} else {
							l.Errorf("failed to parse connect_info for %s: %v", ems.ServerName, err)
						}
					}
					if len(chosenConn.URL) > 0 {
						// 替换 URL 中的 localhost 端口为 serverPort
						url := chosenConn.URL
						// 检查是否包含 localhost，如果包含则需要分配端口
						if strings.Contains(url, "localhost") {
							// 如果端口还未分配，则分配新端口
							if serviceInstall.Port == 0 {
								//查询当前最大端口
								port, err4 := serviceInstallModel.GetMaxPort(ctx)
								if err4 != nil {
									return err4
								}
								if port > 0 {
									serviceInstall.Port = port + 1
								} else {
									serviceInstall.Port = 10000
								}
								l.Infof("Found localhost URL in service %s (ServerId: %s): %s, allocated new port %d", ems.ServerName, mcpService.ServerId, url, serviceInstall.Port)
							} else {
								l.Infof("Found localhost URL in service %s (ServerId: %s): %s, using existing port %d", ems.ServerName, mcpService.ServerId, url, serviceInstall.Port)
							}
							url = replaceLocalhostPort(url, serviceInstall.Port)
						}

						tc := types.TargetConnect{
							URL:            url,
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
				var chosen types.FlatLaunch
				var n types.NestedLaunch
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
						var f types.FlatLaunch
						if err = json.Unmarshal([]byte(ems.LaunchInfo), &f); err == nil {
							chosen = f
						} else {
							l.Errorf("failed to parse launch_info for %s: %v", ems.ServerName, err)
						}
					}
				}
				// 处理 Args 中的 CMD_EXT_PATH 替换
				args := make([]string, len(chosen.Args))
				copy(args, chosen.Args)
				if repositoryName != "" {
					replacePath := "/opt/xldata/McpInstall/" + repositoryName
					for i, arg := range args {
						if strings.Contains(arg, "CMD_EXT_PATH") {
							args[i] = strings.ReplaceAll(arg, "CMD_EXT_PATH", replacePath)
						}
					}
				}
				tl := types.TargetLaunch{
					Command:         chosen.Command,
					Args:            args,
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
				// 组装 Command、Args 和 Env 为 shell 命令，赋值给 PreinstallCmd
				preinstallCmd := buildShellCommand(chosen.Command, args, chosen.Env)
				if preinstallCmd != "" {
					serviceInstall.PreinstallCmd = preinstallCmd
				}
			case "sse":
				serviceConfig.Type = "sse"
				if len(ems.ConnectInfo) > 0 {
					var chosenConn types.FlatConnect
					var nc types.NestedConnect
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
						var fc types.FlatConnect
						if err := json.Unmarshal([]byte(ems.ConnectInfo), &fc); err == nil {
							chosenConn = fc
						} else {
							l.Errorf("failed to parse connect_info for %s: %v", ems.ServerName, err)
						}
					}
					if len(chosenConn.URL) > 0 {
						tc := types.TargetConnect{
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
				err = fmt.Errorf("unknown server type: %s", ems.ServerType)
				return err
			}
			// 判断服务配置是否存在，存在则更新
			if serviceConfig.Id > 0 {
				err = serviceConfigModel.Update(ctx, serviceConfig)
			} else {
				_, err = serviceConfigModel.Insert(ctx, serviceConfig)
			}
			if err != nil {
				l.Errorf("insert config failed: %v", err)
				return err
			}
			// ========================================== 创建/更新MCP安装命令(执行) =======================================================
			// 需要提前安装的，stdio 远程下载的
			if len(serviceInstall.InstallCmd) > 0 || len(serviceInstall.PreinstallCmd) > 0 || len(serviceInstall.Repository) > 0 {
				if serviceInstall.Id > 0 {
					serviceInstall.UpdateTime = time.Now()
					err = serviceInstallModel.Update(ctx, serviceInstall)
				} else {
					_, err = serviceInstallModel.Insert(ctx, serviceInstall)
				}
			}
			// ========================================== 更新外部服务表 ==================================================================
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

func createProjectName(name string) string {
	// 生成项目名 用 @符号+name 去拼接 把name中的空格去掉，然后空格后的首字母改大写
	projectName := strings.ReplaceAll(name, " ", "")
	projectName = strings.Title(projectName)
	return "@" + projectName
}

func createServerId(index int) string {
	// index padded to 5 digits, plus 2-digit random suffix
	rand.Seed(time.Now().UnixNano())
	var randomTwoDigits = rand.Intn(100) // 0-99
	return fmt.Sprintf("%s%05d%02d", "server_", index, randomTwoDigits)
}

// extractProjectDirName 从 git clone 命令中提取项目目录名称
// 例如: "git clone https://github.com/user/repo.git" -> "repo"
// 例如: "git clone git@github.com:user/my-project.git" -> "my-project"
// 例如: "git clone https://github.com/user/repo.git, cd repo, npm install" -> "repo" (只处理第一个命令)
func extractProjectDirName(installCmd string) string {
	// 按逗号分割，只处理第一个命令
	commands := strings.Split(installCmd, ",")
	if len(commands) == 0 {
		return ""
	}
	firstCmd := strings.TrimSpace(commands[0])

	// 移除 "git clone" 前缀（如果存在）
	cmd := firstCmd
	if strings.HasPrefix(cmd, "git clone") {
		cmd = strings.TrimPrefix(cmd, "git clone")
		cmd = strings.TrimSpace(cmd)
	}

	// 移除可能的参数（如 -b branch）
	parts := strings.Fields(cmd)
	var repoURL string
	for i, part := range parts {
		// 跳过参数，找到 URL
		if !strings.HasPrefix(part, "-") && (strings.Contains(part, ".git") || strings.Contains(part, "://") || strings.Contains(part, "@")) {
			repoURL = part
			break
		}
		// 如果没找到，取最后一个非参数部分
		if i == len(parts)-1 && !strings.HasPrefix(part, "-") {
			repoURL = part
		}
	}

	if repoURL == "" {
		return ""
	}

	// 移除 .git 后缀
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// 提取最后一个 / 之后的部分
	lastSlash := strings.LastIndex(repoURL, "/")
	if lastSlash >= 0 && lastSlash < len(repoURL)-1 {
		return repoURL[lastSlash+1:]
	}

	// 如果没有 /，可能是 git@host:user/repo 格式
	colonIndex := strings.LastIndex(repoURL, ":")
	if colonIndex >= 0 && colonIndex < len(repoURL)-1 {
		afterColon := repoURL[colonIndex+1:]
		lastSlash = strings.LastIndex(afterColon, "/")
		if lastSlash >= 0 && lastSlash < len(afterColon)-1 {
			return afterColon[lastSlash+1:]
		}
		return afterColon
	}

	return repoURL
}

// replaceLocalhostPort 替换 URL 中的 localhost 端口为指定端口
func replaceLocalhostPort(url string, port int64) string {
	// 使用正则表达式匹配 http://localhost:端口 或 https://localhost:端口
	// 支持带或不带尾部斜杠和路径
	re := regexp.MustCompile(`(https?://localhost:)\d+`)
	portStr := fmt.Sprintf("%d", port)
	return re.ReplaceAllString(url, "${1}"+portStr)
}

// buildShellCommand 将 Command、Args 和 Env 组装成可执行的 shell 命令
func buildShellCommand(command string, args []string, env map[string]string) string {
	if command == "" {
		return ""
	}

	var parts []string

	// 添加环境变量（如果存在）
	if len(env) > 0 {
		for key, value := range env {
			// 转义特殊字符，确保值被正确引用
			escapedValue := strings.ReplaceAll(value, `"`, `\"`)
			parts = append(parts, fmt.Sprintf("%s=\"%s\"", key, escapedValue))
		}
	}

	// 添加命令
	parts = append(parts, command)

	// 添加参数
	if len(args) > 0 {
		for _, arg := range args {
			// 如果参数包含空格或特殊字符，需要引用
			if strings.ContainsAny(arg, " \t\n\"'$`\\") {
				escapedArg := strings.ReplaceAll(arg, `"`, `\"`)
				parts = append(parts, fmt.Sprintf(`"%s"`, escapedArg))
			} else {
				parts = append(parts, arg)
			}
		}
	}

	return strings.Join(parts, " ")
}
