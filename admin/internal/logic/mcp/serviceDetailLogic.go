package mcp

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"context"
	"errors"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceDetailLogic {
	return &ServiceDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceDetailLogic) ServiceDetail(req *types.DetailReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	serviceDetail, err := l.svcCtx.McpServiceModel.FindOneByCondition(l.ctx, []models.Condition{
		{
			Field: "id",
			Value: req.Id,
		},
	})
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return
	}
	if serviceDetail == nil {
		resp = &types.BaseResp{
			Code:    1,
			Message: "服务不存在",
			Data:    types.D{},
		}
		return
	}
	var chainDetail *configModel.AeMcpTaskChain
	var nodeList []*configModel.AeMcpTaskNode
	chainDetail, err = l.svcCtx.TaskChainModel.FindOne(l.ctx, serviceDetail.TaskChainId)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return
	}
	if chainDetail != nil {
		nodeList, _, err = l.svcCtx.TaskNodeModel.GetList(l.ctx, models.ListConditions{
			Conditions: []models.Condition{
				{
					Field:  "id",
					Symbol: "in",
					Value:  chainDetail.NodeIds,
				},
			},
		}, true)
		if err != nil {
			return
		}

	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"service":   serviceDetail,
			"chain":     chainDetail,
			"node_list": nodeList,
		},
	}
	err = nil
	return
}
