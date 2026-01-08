// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/data"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetServiceConfigListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetServiceConfigListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := data.NewGetServiceConfigListLogic(r.Context(), svcCtx)
		resp, err := l.GetServiceConfigList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
