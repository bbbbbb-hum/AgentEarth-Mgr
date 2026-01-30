package data

import (
	"AgentEarth-Mgr/models"
	"context"
	"errors"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type BatchCloseServicesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchCloseServicesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchCloseServicesLogic {
	return &BatchCloseServicesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchCloseServicesLogic) BatchCloseServices(req *types.BatchCloseServicesReq) (resp *types.BaseResp, err error) {
	// 获取外部配置列表
	externalMcpServices, total, err := l.svcCtx.ExternalMcpServicesModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field: "test_status",
				Value: req.TestStatus,
			},
		},
	}, true)
	if err != nil {
		return nil, err
	}
	var updateNumber int64
	if total > 0 {
		for _, ems := range externalMcpServices {
			// 判断服务是否存在，存在则更新
			mcpService, err2 := l.svcCtx.McpServiceModel.FindOneByCondition(l.ctx, []models.Condition{
				{
					Field: "server_name",
					Value: ems.ServerName,
				},
			})
			if err2 != nil && !errors.Is(err2, sqlx.ErrNotFound) {
				logx.Error(err2)
				continue
			}
			if mcpService != nil {
				mcpService.UpdateTime = time.Now()
				mcpService.Enabled = false
			}
			err2 = l.svcCtx.McpServiceModel.Update(l.ctx, mcpService)
			if err2 != nil {
				logx.Error(err2)
			}
			updateNumber++
		}
	}

	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"total":         total,
			"update_number": updateNumber,
		},
	}

	return
}
