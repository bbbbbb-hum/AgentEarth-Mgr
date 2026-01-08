// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package source

import (
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/source"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ServiceSyncTestToProdHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.IdsReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := source.NewServiceSyncTestToProdLogic(r.Context(), svcCtx)
		resp, err := l.ServiceSyncTestToProd(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
