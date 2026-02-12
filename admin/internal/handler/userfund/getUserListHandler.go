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

// 这一整段代码(从func(w, r)开始到最后一个}结束)就是一个“匿名函数”的定义
// 而这个匿名函数本身，就是GetUserListHandler的返回值
// 外层函数是工厂，内层是工人---闭包
func GetUserListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//1.解析参数
		var req types.UserListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		//2.真正调用logic层的业务逻辑
		l := userfund.NewGetUserListLogic(r.Context(), svcCtx)
		resp, err := l.GetUserList(&req)
		//3.返回结果
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
