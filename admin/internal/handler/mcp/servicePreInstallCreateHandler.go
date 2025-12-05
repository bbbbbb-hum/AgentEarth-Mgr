package mcp

import (
	"archive/zip"
	"bytes"
	"net/http"
	"strconv"
	"time"

	"AgentEarth-Mgr/admin/internal/logic/mcp"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ServicePreInstallCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ServicePreInstallCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := mcp.NewServicePreInstallCreateLogic(r.Context(), svcCtx)
		startScript, err := l.ServicePreInstallCreate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// 生成停止脚本
		stopScript := mcp.GenerateStopScript()

		// 创建 ZIP 文件
		var zipBuffer bytes.Buffer
		zipWriter := zip.NewWriter(&zipBuffer)

		// 添加启动脚本
		startFilename := "preinstall_" + time.Now().Format("20060102_150405") + ".sh"
		startFile, err := zipWriter.Create(startFilename)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		_, _ = startFile.Write([]byte(startScript))

		// 添加停止脚本
		stopFilename := "stop_all_preinstall.sh"
		stopFile, err := zipWriter.Create(stopFilename)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		_, _ = stopFile.Write([]byte(stopScript))

		// 关闭 ZIP writer
		if err := zipWriter.Close(); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// 设置文件下载响应头
		zipFilename := "preinstall_scripts_" + time.Now().Format("20060102_150405") + ".zip"
		content := zipBuffer.Bytes()
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename="+zipFilename)
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))

		// 写入文件内容
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}
}
