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

type ServicePreInstallCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServicePreInstallCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServicePreInstallCreateLogic {
	return &ServicePreInstallCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServicePreInstallCreateLogic) ServicePreInstallCreate(req *types.ServicePreInstallCreateReq) (shellContent string, err error) {
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
	sb.WriteString("# Auto-generated preinstall script\n")
	sb.WriteString("# Generated at: " + time.Now().Format("2006-01-02 15:04:05") + "\n\n")

	// PID 文件路径，使用固定文件名便于管理
	sb.WriteString("PID_FILE=\"/tmp/preinstall_pids.txt\"\n")
	sb.WriteString("PIDS=()\n")
	sb.WriteString("# Clear existing PID file at start\n")
	sb.WriteString("> \"$PID_FILE\"\n\n")

	// 添加停止所有进程的函数
	sb.WriteString("# Function to stop all preinstall processes\n")
	sb.WriteString("# Usage: stop_all_preinstall [PID_FILE_PATH]\n")
	sb.WriteString("stop_all_preinstall() {\n")
	sb.WriteString("    local pid_file=\"${1:-$PID_FILE}\"\n")
	sb.WriteString("    if [ -f \"$pid_file\" ]; then\n")
	sb.WriteString("        echo 'Stopping all preinstall processes...'\n")
	sb.WriteString("        local stopped=0\n")
	sb.WriteString("        while IFS= read -r pid; do\n")
	sb.WriteString("            if [ -n \"$pid\" ] && kill -0 \"$pid\" 2>/dev/null; then\n")
	sb.WriteString("                echo \"Stopping process PID: $pid\"\n")
	sb.WriteString("                kill \"$pid\" 2>/dev/null && stopped=$((stopped + 1)) || true\n")
	sb.WriteString("            fi\n")
	sb.WriteString("        done < \"$pid_file\"\n")
	sb.WriteString("        echo \"Stopped $stopped process(es)\"\n")
	sb.WriteString("        echo \"PID file: $pid_file (not removed, you can delete it manually)\"\n")
	sb.WriteString("    else\n")
	sb.WriteString("        echo \"No PID file found at: $pid_file\"\n")
	sb.WriteString("        echo 'No processes to stop'\n")
	sb.WriteString("    fi\n")
	sb.WriteString("}\n\n")

	commandIndex := 1
	for i, item := range list {
		preinstallCmd := strings.TrimSpace(item.PreinstallCmd)
		if preinstallCmd == "" {
			continue
		}

		// 为每个服务添加注释
		sb.WriteString(fmt.Sprintf("# ========== Service %d - Server ID: %s ==========\n", i+1, item.ServerId))
		sb.WriteString(fmt.Sprintf("# [%d] Start preinstall command\n", commandIndex))

		// 使用 nohup 和 & 让命令在后台运行，避免阻塞
		// 同时将输出重定向到日志文件
		logFilePattern := fmt.Sprintf("/tmp/preinstall_%s_$(date +%%s).log", item.ServerId)
		sb.WriteString(fmt.Sprintf("LOG_FILE=\"%s\"\n", logFilePattern))
		sb.WriteString(fmt.Sprintf("nohup %s > \"$LOG_FILE\" 2>&1 &\n", preinstallCmd))
		sb.WriteString("PREINSTALL_PID=$!\n")
		sb.WriteString("PIDS+=($PREINSTALL_PID)\n")
		sb.WriteString("echo \"$PREINSTALL_PID\" >> \"$PID_FILE\"\n")
		sb.WriteString(fmt.Sprintf("echo 'Started preinstall command for %s, PID: $PREINSTALL_PID, log: $LOG_FILE'\n", item.ServerId))
		sb.WriteString("\n")
		commandIndex++
	}

	if commandIndex == 1 {
		// 没有找到任何预安装命令
		sb.WriteString("# No preinstall commands found\n")
		sb.WriteString("echo 'No preinstall commands to execute'\n")
	} else {
		// Wait a moment for services to start
		sb.WriteString("sleep 2\n")
		sb.WriteString("echo 'All preinstall commands have been started'\n")
		sb.WriteString("echo \"PID file saved to: $PID_FILE\"\n")
		sb.WriteString("echo 'To stop all processes, run: bash stop_all_preinstall.sh'\n")
		sb.WriteString("echo 'Or manually kill processes listed in the PID file'\n")
	}

	return sb.String(), nil
}

// GenerateStopScript 生成独立的停止脚本
func GenerateStopScript() string {
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# Auto-generated stop script for preinstall processes\n")
	sb.WriteString("# Generated at: " + time.Now().Format("2006-01-02 15:04:05") + "\n\n")

	// 使用固定的 PID 文件路径
	sb.WriteString("# Stop all preinstall processes\n")
	sb.WriteString("PID_FILE=\"/tmp/preinstall_pids.txt\"\n")
	sb.WriteString("if [ -n \"$1\" ]; then\n")
	sb.WriteString("    # Use provided PID file path\n")
	sb.WriteString("    PID_FILE=\"$1\"\n")
	sb.WriteString("fi\n\n")

	sb.WriteString("if [ ! -f \"$PID_FILE\" ]; then\n")
	sb.WriteString("    echo \"PID file not found: $PID_FILE\"\n")
	sb.WriteString("    exit 1\n")
	sb.WriteString("fi\n\n")

	sb.WriteString("echo \"Using PID file: $PID_FILE\"\n")
	sb.WriteString("echo 'Stopping all preinstall processes...'\n\n")

	sb.WriteString("stopped=0\n")
	sb.WriteString("failed=0\n")
	sb.WriteString("while IFS= read -r pid; do\n")
	sb.WriteString("    if [ -z \"$pid\" ]; then\n")
	sb.WriteString("        continue\n")
	sb.WriteString("    fi\n")
	sb.WriteString("    if kill -0 \"$pid\" 2>/dev/null; then\n")
	sb.WriteString("        echo \"Stopping process PID: $pid\"\n")
	sb.WriteString("        if kill \"$pid\" 2>/dev/null; then\n")
	sb.WriteString("            stopped=$((stopped + 1))\n")
	sb.WriteString("        else\n")
	sb.WriteString("            failed=$((failed + 1))\n")
	sb.WriteString("            echo \"  Failed to stop PID: $pid\"\n")
	sb.WriteString("        fi\n")
	sb.WriteString("    else\n")
	sb.WriteString("        echo \"Process PID $pid is not running (already stopped)\"\n")
	sb.WriteString("    fi\n")
	sb.WriteString("done < \"$PID_FILE\"\n\n")

	sb.WriteString("echo \"\"\n")
	sb.WriteString("echo \"Summary:\"\n")
	sb.WriteString("echo \"  Stopped: $stopped process(es)\"\n")
	sb.WriteString("if [ $failed -gt 0 ]; then\n")
	sb.WriteString("    echo \"  Failed: $failed process(es)\"\n")
	sb.WriteString("fi\n")
	sb.WriteString("echo \"PID file: $PID_FILE (not removed, you can delete it manually)\"\n")

	return sb.String()
}
