package data

import (
	"errors"
	"net/http"
	"strings"

	"AgentEarth-Mgr/admin/internal/logic/data"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetTaskNodeNodeConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetTaskNodeNodeConfigReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if strings.TrimSpace(req.ServerId) == "" {
			httpx.ErrorCtx(r.Context(), w, errors.New("field \"server_id\" is not set"))
			return
		}

		l := data.NewGetTaskNodeNodeConfigLogic(r.Context(), svcCtx)
		resp, err := l.GetTaskNodeNodeConfig(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

