// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/data"
	"AgentEarth-Mgr/admin/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateServicesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := data.NewCreateServicesLogic(r.Context(), svcCtx)
		resp, err := l.CreateServices()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
