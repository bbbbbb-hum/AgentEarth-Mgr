package userfund

import (
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/userfund"
	"AgentEarth-Mgr/admin/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func TestExpireCheckHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 无请求参数，直接调用逻辑
		l := userfund.NewTestExpireCheckLogic(r.Context(), svcCtx)
		resp, err := l.TestExpireCheck()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
