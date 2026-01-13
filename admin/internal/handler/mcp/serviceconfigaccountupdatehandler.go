// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package mcp

import (
	"encoding/json"
	"errors"
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/mcp"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ServiceConfigAccountUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ServiceConfigAccountUpdateReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if req.Id == 0 {
			httpx.ErrorCtx(r.Context(), w, errors.New("field \"id\" is not set"))
			return
		}
		if req.Name == "" {
			httpx.ErrorCtx(r.Context(), w, errors.New("field \"name\" is not set"))
			return
		}
		if req.Status == "" {
			httpx.ErrorCtx(r.Context(), w, errors.New("field \"status\" is not set"))
			return
		}

		l := mcp.NewServiceConfigAccountUpdateLogic(r.Context(), svcCtx)
		resp, err := l.ServiceConfigAccountUpdate(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
