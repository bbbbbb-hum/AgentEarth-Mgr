package data

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"AgentEarth-Mgr/admin/internal/logic/data"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func UpdateTaskNodeNodeConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateTaskNodeNodeConfigReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if len(strings.TrimSpace(req.ServerId)) == 0 {
			httpx.ErrorCtx(r.Context(), w, errors.New("field \"server_id\" is not set"))
			return
		}

		l := data.NewUpdateTaskNodeNodeConfigLogic(r.Context(), svcCtx)
		resp, err := l.UpdateTaskNodeNodeConfig(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

