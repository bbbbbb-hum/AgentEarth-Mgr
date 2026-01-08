package mcp

import (
	"AgentEarth-Mgr/models"
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceConfigAccountListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceConfigAccountListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceConfigAccountListLogic {
	return &ServiceConfigAccountListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceConfigAccountListLogic) ServiceConfigAccountList(req *types.ServiceConfigAccountListReq) (resp *types.BaseResp, err error) {
	var conditions []models.Condition
	if len(req.Search) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "name",
			Symbol: "ILIKE",
			Value:  "%" + req.Search + "%",
		})
	}
	if req.ConfigId > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "config_id",
			Symbol: "=",
			Value:  req.ConfigId,
		})
	}
	list, total, err := l.svcCtx.TaskNodeConfigAccountModel.GetList(l.ctx, models.ListConditions{
		Conditions: conditions,
		Pages: models.Pages{
			Page: req.Page,
			Size: req.Size,
		},
	}, true)
	if err != nil {
		return
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"list": list,
			"total": total,
		},
	}
	return
}
