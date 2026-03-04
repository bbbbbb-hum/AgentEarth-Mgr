// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package userfund

import (
	"net/http"

	"AgentEarth-Mgr/admin/internal/logic/userfund"
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func ManualDeductionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ManualDeductionReq
		if err := httpx.ParseJsonBody(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// 从 Authorization 中解析管理员身份，写入上下文，便于后续记录 operator
		opCtx := withOperatorUsernameCtx(r, svcCtx)
		l := userfund.NewManualDeductionLogic(opCtx, svcCtx)
		resp, err := l.ManualDeduction(&req)
		if err != nil {
			httpx.ErrorCtx(opCtx, w, err)
		} else {
			httpx.OkJsonCtx(opCtx, w, resp)
		}
	}
}
