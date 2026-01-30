package source

import (
	"AgentEarth-Mgr/models"
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogic {
	return &ListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListLogic) List(req *types.SourceListReq) (resp *types.BaseResp, err error) {
	listConditions := models.ListConditions{
		Conditions: []models.Condition{},
		Pages: models.Pages{
			Page: req.Page,
			Size: req.Size,
		},
		Sorts: []models.Sort{
			{
				Filed: "create_time",
				Order: "desc",
			},
		},
	}
	if len(req.Search) > 0 {
		listConditions.Conditions = append(listConditions.Conditions, models.Condition{
			Field:  "server_name",
			Symbol: "ILIKE",
			Value:  "%" + req.Search + "%",
		})
	}
	if len(req.TestStatus) > 0 {
		listConditions.Conditions = append(listConditions.Conditions, models.Condition{
			Field:  "test_status",
			Symbol: "=",
			Value:  req.TestStatus,
		})
	}
	if len(req.ServerType) > 0 {
		listConditions.Conditions = append(listConditions.Conditions, models.Condition{
			Field:  "server_type",
			Symbol: "=",
			Value:  req.ServerType,
		})
	}
	list, total, err := l.svcCtx.ExternalMcpServicesModel.GetList(l.ctx, listConditions, true)
	if err != nil {
		return
	}
	resp = &types.BaseResp{
		Code: 0,
		Data: types.D{
			"list":  list,
			"total": total,
		},
		Message: "success",
	}
	return
}
