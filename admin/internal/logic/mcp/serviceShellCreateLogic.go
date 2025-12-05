package mcp

import (
	"AgentEarth-Mgr/models"
	"context"
	"fmt"
	"strings"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceShellCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceShellCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceShellCreateLogic {
	return &ServiceShellCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceShellCreateLogic) ServiceShellCreate(req *types.ServiceShellCreateReq) (shellContent string, err error) {
	list, _, err := l.svcCtx.McpServicesInstallModel.GetList(l.ctx, models.ListConditions{
		Pages: models.Pages{},
		Conditions: []models.Condition{
			{
				Field:  "id",
				Symbol: "IN",
				Value:  req.InstallIds,
			},
		},
		Sorts: nil,
	}, true)
	if err != nil {
		return "", err
	}

	// 构建 shell 脚本内容
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# Auto-generated install script\n")
	sb.WriteString("# Generated at: " + time.Now().Format("2006-01-02 15:04:05") + "\n\n")

	// 检查是否需要安装 pm2（只有 Port > 0 且最后一条命令是启动命令时才需要）
	needPm2 := false
	for _, item := range list {
		if strings.TrimSpace(item.Repository) != "" && strings.TrimSpace(item.InstallCmd) != "" && item.Port > 0 {
			commands := strings.Split(item.InstallCmd, ",")
			if len(commands) > 0 {
				lastCmd := strings.TrimSpace(commands[len(commands)-1])
				if isStartCommand(lastCmd) {
					needPm2 = true
					break
				}
			}
		}
	}

	// 如果需要 pm2，先安装
	if needPm2 {
		sb.WriteString("# Install PM2 globally\n")
		sb.WriteString("if ! command -v pm2 &> /dev/null; then\n")
		sb.WriteString("    echo 'Installing PM2...'\n")
		sb.WriteString("    npm install -g pm2\n")
		sb.WriteString("else\n")
		sb.WriteString("    echo 'PM2 is already installed'\n")
		sb.WriteString("fi\n\n")
	}

	commandIndex := 1
	for i, item := range list {
		// 检查 Repository 和 InstallCmd 是否都不为空
		repository := strings.TrimSpace(item.Repository)
		installCmd := strings.TrimSpace(item.InstallCmd)

		if repository == "" || installCmd == "" {
			continue
		}

		// 为每个服务添加注释
		sb.WriteString(fmt.Sprintf("# ========== Service %d - Server ID: %s ==========\n", i+1, item.ServerId))

		// 从 Repository 中提取项目目录名称
		projectDirName := extractProjectDirName(repository)

		// 第一步：git clone Repository（如果目录不存在）
		sb.WriteString(fmt.Sprintf("# [%d] Clone repository\n", commandIndex))
		if projectDirName != "" {
			sb.WriteString(fmt.Sprintf("if [ ! -d \"%s\" ]; then\n", projectDirName))
			sb.WriteString(fmt.Sprintf("    git clone %s\n", repository))
			sb.WriteString("else\n")
			sb.WriteString(fmt.Sprintf("    echo 'Directory %s already exists, skipping git clone'\n", projectDirName))
			sb.WriteString("fi\n")
		} else {
			sb.WriteString(fmt.Sprintf("git clone %s", repository))
		}
		sb.WriteString("\n\n")
		commandIndex++

		// 按逗号分割 InstallCmd，支持多个命令
		commands := strings.Split(installCmd, ",")

		// 判断最后一条命令是否是启动命令
		var lastCmd string
		var isStart bool
		if len(commands) > 0 {
			lastCmd = strings.TrimSpace(commands[len(commands)-1])
			isStart = item.Port > 0 && isStartCommand(lastCmd)
		}

		// 处理所有命令
		for j := 0; j < len(commands); j++ {
			cmd := strings.TrimSpace(commands[j])
			if cmd == "" {
				continue
			}

			// 判断是否是最后一条命令且需要启动
			isLastCmd := (j == len(commands)-1)

			var stepDesc string
			var finalCmd string

			if isLastCmd && isStart {
				// 最后一条命令且需要启动，使用 pm2
				stepDesc = "Start service with PM2"
				finalCmd = convertToPm2Command(cmd, item.ServerId, projectDirName, item.Port)
			} else {
				// 普通命令（安装/构建），直接执行
				if strings.Contains(cmd, "install") {
					stepDesc = "Install dependencies"
				} else if strings.Contains(cmd, "build") {
					stepDesc = "Build project"
				} else {
					stepDesc = fmt.Sprintf("Step %d", j+2)
				}
				finalCmd = cmd
			}

			sb.WriteString(fmt.Sprintf("# [%d] %s\n", commandIndex, stepDesc))
			if projectDirName != "" {
				// 使用子 shell 执行命令，避免影响后续命令的工作目录
				sb.WriteString(fmt.Sprintf("(cd %s && %s)", projectDirName, finalCmd))
			} else {
				sb.WriteString(finalCmd)
			}
			sb.WriteString("\n\n")
			commandIndex++
		}

		sb.WriteString("\n")
	}

	// 如果使用了 pm2，添加保存配置
	if needPm2 {
		sb.WriteString("# Save PM2 process list\n")
		sb.WriteString("pm2 save\n\n")
	}

	return sb.String(), nil
}

// extractProjectDirName 从 Repository URL 中提取项目目录名称
func extractProjectDirName(repository string) string {
	if repository == "" {
		return ""
	}

	repoURL := strings.TrimSpace(repository)

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

// isStartCommand 判断命令是否是启动命令
func isStartCommand(cmd string) bool {
	cmdLower := strings.ToLower(cmd)
	// 启动命令通常包含：dev, start, serve 等关键词
	return strings.Contains(cmdLower, "dev") ||
		strings.Contains(cmdLower, "start") ||
		strings.Contains(cmdLower, "serve") ||
		strings.HasPrefix(cmdLower, "node ")
}

// convertToPm2Command 将启动命令转换为 pm2 命令，并添加端口号
func convertToPm2Command(startCmd, serverId, projectDirName string, port int64) string {
	appName := serverId
	if projectDirName != "" {
		appName = projectDirName + "-" + serverId
	}

	// 构建端口环境变量
	portEnv := ""
	if port > 0 {
		portEnv = fmt.Sprintf("PORT=%d ", port)
	}

	// 如果命令是 "npm start" 或类似格式
	if strings.HasPrefix(startCmd, "npm") {
		// 提取 npm 后面的参数
		parts := strings.Fields(startCmd)
		if len(parts) >= 2 {
			// pm2 start npm --name "app" -- start (带端口环境变量)
			args := strings.Join(parts[1:], " ")
			if portEnv != "" {
				return fmt.Sprintf("%spm2 start npm --name \"%s\" -- %s", portEnv, appName, args)
			}
			return fmt.Sprintf("pm2 start npm --name \"%s\" -- %s", appName, args)
		}
	}

	// 如果命令是 "node app.js" 或类似格式
	if strings.HasPrefix(startCmd, "node") {
		parts := strings.Fields(startCmd)
		if len(parts) >= 2 {
			scriptPath := strings.Join(parts[1:], " ")
			if portEnv != "" {
				return fmt.Sprintf("%spm2 start %s --name \"%s\"", portEnv, scriptPath, appName)
			}
			return fmt.Sprintf("pm2 start %s --name \"%s\"", scriptPath, appName)
		}
	}

	// 其他情况，使用 pm2 start 包装整个命令
	if portEnv != "" {
		return fmt.Sprintf("%spm2 start \"%s\" --name \"%s\"", portEnv, startCmd, appName)
	}
	return fmt.Sprintf("pm2 start \"%s\" --name \"%s\"", startCmd, appName)
}
