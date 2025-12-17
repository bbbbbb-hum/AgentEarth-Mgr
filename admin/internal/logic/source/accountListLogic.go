package source

import (
	"AgentEarth-Mgr/models"
	"context"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AccountListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAccountListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountListLogic {
	return &AccountListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AccountListLogic) AccountList(req *types.AccountListReq) (resp *types.BaseResp, err error) {
	var conditions []models.Condition
	if len(req.Search) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "name",
			Symbol: "ILIKE",
			Value:  "%" + req.Search + "%",
		})
	}
	if req.SourceId > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "external_mcp_services_id",
			Symbol: "=",
			Value:  req.SourceId,
		})
	}
	list, total, err := l.svcCtx.ExternalMpcServicesAccountModel.GetList(l.ctx, models.ListConditions{
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
		Code: 0,
		Data: types.D{
			"list":  list,
			"total": total,
		},
		Message: "success",
	}
	return
}
