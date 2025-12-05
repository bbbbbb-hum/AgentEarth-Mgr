package mcp

import (
	"net/http"
	"strconv"
	"time"

	"AgentEarth-Mgr/admin/internal/logic/mcp"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ServiceShellCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ServiceShellCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := mcp.NewServiceShellCreateLogic(r.Context(), svcCtx)
		shellContent, err := l.ServiceShellCreate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// 设置文件下载响应头
		filename := "install_" + time.Now().Format("20060102_150405") + ".sh"
		content := []byte(shellContent)
		w.Header().Set("Content-Type", "application/x-sh")
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))

		// 写入文件内容
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}
}
