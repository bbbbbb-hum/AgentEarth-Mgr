package source

import (
	"AgentEarth-Mgr/models"
	"AgentEarth-Mgr/models/external"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"AgentEarth-Mgr/admin/internal/logic/data"
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
	for _, ems := range externalMcpServices {
		err := l.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
			sessConn := sqlx.NewSqlConnFromSession(session)
			serviceModel := mcp.NewAeMcpServicesModel(sessConn)
			externalMcpServiceModel := external.NewExternalMcpServicesModel(sessConn)
			serviceInstallModel := mcp.NewAeMcpServicesInstallModel(sessConn)

			var installCmd, repositoryUrl string
			projectName := ems.ProjectName.String
			if !ems.ProjectName.Valid {
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
				id, err := data.NextServiceId(ctx, sessConn)
				if err != nil {
					return err
				}
				serverId := data.FormatServerId(id)
				mcpService = &mcp.AeMcpServices{
					Id:              id,
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
			}
			//判断更新还是插入
			mcpService.IsCreated = true
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
			// 安装命令，仓库，预安装命令字段以外部配置表为准，如果外部配置表无值则置空
			serviceInstall.InstallCmd = installCmd
			serviceInstall.Repository = repositoryUrl
			// ========================================== 创建/更新MCP安装命令(执行) =======================================================
			// 需要提前安装的，stdio 远程下载的
			if len(serviceInstall.InstallCmd) > 0 || len(serviceInstall.PreinstallCmd) > 0 || len(serviceInstall.Repository) > 0 {
				var installErr error
				if serviceInstall.Id > 0 {
					serviceInstall.UpdateTime = time.Now()
					installErr = serviceInstallModel.Update(ctx, serviceInstall)
				} else {
					_, installErr = serviceInstallModel.Insert(ctx, serviceInstall)
				}
				if installErr != nil {
					return installErr
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

func pickFlatConnect(connectInfo, serverName string, logger logx.Logger) (types.FlatConnect, error) {
	var chosen types.FlatConnect
	if len(connectInfo) == 0 {
		return chosen, nil
	}

	// 优先尝试 NestedConnect（兼容 {"mcpServers": {...}}）
	var nc types.NestedConnect
	if err := json.Unmarshal([]byte(connectInfo), &nc); err == nil && len(nc.McpServers) > 0 {
		if v, ok := nc.McpServers[serverName]; ok {
			return v, nil
		}
		for _, v := range nc.McpServers {
			return v, nil
		}
		return chosen, nil
	}

	// 兼容 FlatConnect（直接就是一个 connect 配置）
	var fc types.FlatConnect
	if err := json.Unmarshal([]byte(connectInfo), &fc); err == nil {
		return fc, nil
	} else {
		logger.Errorf("failed to parse connect_info for %s: %v", serverName, err)
		return chosen, err
	}
}

// applyConnectInfoWithLocalhost 已移除：依赖 ae_mcp_external_services_config（旧表），且从未被调用
