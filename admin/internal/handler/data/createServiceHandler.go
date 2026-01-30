// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"encoding/json"
	"io"
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/data"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateServiceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateServiceReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := data.NewCreateServiceLogic(r.Context(), svcCtx)
		resp, err := l.CreateService(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
