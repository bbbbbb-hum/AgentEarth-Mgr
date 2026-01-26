// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package userfund

import (
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/userfund"
	"AgentEarth-Mgr/admin/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := userfund.NewGetStatsLogic(r.Context(), svcCtx)
		resp, err := l.GetStats()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
