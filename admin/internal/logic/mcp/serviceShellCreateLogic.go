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

// bashEscapeForDoubleQuotes 将字符串转义为可安全放入 bash 双引号中的内容
// 主要避免 \"、\\、$、` 破坏 echo 行本身的语法或触发替换。
func bashEscapeForDoubleQuotes(s string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"$", "\\$",
		"`", "\\`",
	)
	return replacer.Replace(s)
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
	// 生产脚本仅负责拉取代码并执行安装/构建等命令（不负责启动服务）

	commandIndex := 1
	for i, item := range list {
		// 检查 Repository 和 InstallCmd 是否都不为空
		repository := strings.TrimSpace(item.Repository)
		installCmd := strings.TrimSpace(item.InstallCmd)

		if len(repository) == 0 || len(installCmd) == 0 {
			continue
		}

		// 为每个服务添加注释
		sb.WriteString(fmt.Sprintf("# ========== Service %d - Server ID: %s ==========\n", i+1, item.ServerId))

		// 从 Repository 中提取项目目录名称
		projectDirName := extractProjectDirName(repository)

		// 第一步：git clone Repository（如果目录不存在）
		sb.WriteString(fmt.Sprintf("# [%d] Clone repository\n", commandIndex))
		if len(projectDirName) > 0 {
			sb.WriteString(fmt.Sprintf("if [ ! -d \"%s\" ]; then\n", projectDirName))
			sb.WriteString(fmt.Sprintf("    echo \"[%d] Running: git clone %s\"\n", commandIndex, bashEscapeForDoubleQuotes(repository)))
			sb.WriteString(fmt.Sprintf("    git clone %s\n", repository))
			sb.WriteString("else\n")
			sb.WriteString(fmt.Sprintf("    echo 'Directory %s already exists, skipping git clone'\n", projectDirName))
			sb.WriteString("fi\n")
		} else {
			sb.WriteString(fmt.Sprintf("echo \"[%d] Running: git clone %s\"\n", commandIndex, bashEscapeForDoubleQuotes(repository)))
			sb.WriteString(fmt.Sprintf("git clone %s", repository))
		}
		sb.WriteString("\n\n")
		commandIndex++

		// 按逗号分割 InstallCmd，支持多个命令
		commands := strings.Split(installCmd, ",")

		// 处理所有命令
		for j := 0; j < len(commands); j++ {
			cmd := strings.TrimSpace(commands[j])
			if len(cmd) == 0 {
				continue
			}

			var stepDesc string
			var finalCmd string

			// 普通命令（安装/构建），直接执行
			if strings.Contains(cmd, "install") {
				stepDesc = "Install dependencies"
			} else if strings.Contains(cmd, "build") {
				stepDesc = "Build project"
			} else {
				stepDesc = fmt.Sprintf("Step %d", j+2)
			}
			finalCmd = cmd

			sb.WriteString(fmt.Sprintf("# [%d] %s\n", commandIndex, stepDesc))
			if len(projectDirName) > 0 {
				// 使用子 shell 执行命令，避免影响后续命令的工作目录
				sb.WriteString(fmt.Sprintf("(cd %s && echo \"[%d] Running: %s\" && %s)", projectDirName, commandIndex, bashEscapeForDoubleQuotes(finalCmd), finalCmd))
			} else {
				sb.WriteString(fmt.Sprintf("echo \"[%d] Running: %s\"\n", commandIndex, bashEscapeForDoubleQuotes(finalCmd)))
				sb.WriteString(finalCmd)
			}
			sb.WriteString("\n\n")
			commandIndex++
		}

		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// extractProjectDirName 从 Repository URL 中提取项目目录名称
func extractProjectDirName(repository string) string {
	if len(repository) == 0 {
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
