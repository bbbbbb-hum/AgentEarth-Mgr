// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"encoding/json"
	"errors"
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/data"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func UpdateServiceConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateServiceConfigReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.Id == 0 {
			httpx.ErrorCtx(r.Context(), w, errors.New("field \"id\" is not set"))
			return
		}

		l := data.NewUpdateServiceConfigLogic(r.Context(), svcCtx)
		resp, err := l.UpdateServiceConfig(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
