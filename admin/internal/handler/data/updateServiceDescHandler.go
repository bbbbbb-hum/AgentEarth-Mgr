package data

import (
	"AgentEarth-Mgr/admin/internal/types"
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/data"
	"AgentEarth-Mgr/admin/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func UpdateServiceDescHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateServiceDescReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		l := data.NewUpdateServiceDescLogic(r.Context(), svcCtx)
		resp, err := l.UpdateServiceDesc(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
