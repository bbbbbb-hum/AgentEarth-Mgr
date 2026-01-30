package mcp

import (
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/mcp"
	"AgentEarth-Mgr/admin/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func InnerMcpInfoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serverId := r.URL.Query().Get("server_id")
		l := mcp.NewInnerMcpInfoLogic(r.Context(), svcCtx)
		resp, err := l.InnerMcpInfo(serverId)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

