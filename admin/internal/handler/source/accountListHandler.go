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

func AccountListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AccountListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := source.NewAccountListLogic(r.Context(), svcCtx)
		resp, err := l.AccountList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
