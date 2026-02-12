package mcp

import (
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/models"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceConfigDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceConfigDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceConfigDeleteLogic {
	return &ServiceConfigDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceConfigDeleteLogic) ServiceConfigDelete(req *types.ServiceConfigDeleteReq) (resp *types.BaseResp, err error) {
	err = l.svcCtx.TaskNodeConfigV2Model.DeleteByConditions(l.ctx, []models.Condition{
		{
			Field:  "id",
			Symbol: "IN",
			Value:  req.Ids,
		},
	})
	if err != nil {
		return &types.BaseResp{
			Code:    -1,
			Message: "删除失败: " + err.Error(),
		}, nil
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "删除成功",
		Data:    nil,
	}
	return
}
