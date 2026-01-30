// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package mcp

import (
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/mcp"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ServiceBatchCloseHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ServiceBatchCloseReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := mcp.NewServiceBatchCloseLogic(r.Context(), svcCtx)
		resp, err := l.ServiceBatchClose(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
