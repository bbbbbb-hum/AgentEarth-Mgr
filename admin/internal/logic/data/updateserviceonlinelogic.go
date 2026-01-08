// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package data

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateServiceOnlineLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateServiceOnlineLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateServiceOnlineLogic {
	return &UpdateServiceOnlineLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateServiceOnlineLogic) UpdateServiceOnline(req *types.UpdateServiceOnlineReq) (resp *types.BaseResp, err error) {
	service, err := l.svcCtx.TaskNodeConfigModel.FindOne(l.ctx, req.Id)
	if err != nil {
		logx.Errorf("Failed to find service config: %v", err)
		return &types.BaseResp{
			Code:    -1,
			Message: "服务不存在",
		}, nil
	}

	service.OnlineStatus.Valid = true
	service.OnlineStatus.Int64 = req.OnlineStatus

	err = l.svcCtx.TaskNodeConfigModel.Update(l.ctx, service)
	if err != nil {
		logx.Errorf("Failed to update service online status: %v", err)
		return &types.BaseResp{
			Code:    -1,
			Message: "更新服务上线状态失败: " + err.Error(),
		}, nil
	}

	return &types.BaseResp{
		Code:    0,
		Message: "success",
	}, nil
}
