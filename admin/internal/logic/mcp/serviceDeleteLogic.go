package mcp

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"AgentEarth-Mgr/models/mcp"
	"context"
	"errors"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceDeleteLogic {
	return &ServiceDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceDeleteLogic) ServiceDelete(req *types.ServiceDeleteReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	list, total, err := l.svcCtx.McpServiceModel.GetList(l.ctx, models.ListConditions{
		Conditions: []models.Condition{
			{
				Field:  "id",
				Symbol: "IN",
				Value:  req.Ids,
			},
		},
	}, true)
	if err != nil {
		return
	}
	var num int
	if total > 0 {
		for _, service := range list {
			err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
				sessConn := sqlx.NewSqlConnFromSession(session)
				serviceModel := mcp.NewAeMcpServicesModel(sessConn)
				serviceConfigModel := configModel.NewAeMcpExternalServicesConfigModel(sessConn)
				taskChainModel := configModel.NewAeMcpTaskChainModel(sessConn)
				taskNodeModel := configModel.NewAeMcpTaskNodeModel(sessConn)

				taskChain, err1 := taskChainModel.FindOne(l.ctx, service.TaskChainId)
				if err1 == nil && taskChain != nil {
					// 删除任务节点
					err2 := taskNodeModel.DeleteByConditions(l.ctx, []models.Condition{
						{
							Field:  "id",
							Symbol: "IN",
							Value:  taskChain.NodeIds,
						},
					})
					if err2 != nil {
						return err2
					}
					// 删除任务链
					err2 = taskChainModel.Delete(l.ctx, taskChain.Id)
					if err2 != nil {
						return err2
					}
				}
				// 删除服务配置
				err1 = serviceConfigModel.DeleteByConditions(ctx, []models.Condition{
					{
						Field: "server_id",
						Value: service.ServerId,
					},
				})
				if err1 != nil {
					return err1
				}
				// 删除安装命令
				err1 = l.svcCtx.McpServicesInstallModel.DeleteByConditions(ctx, []models.Condition{
					{
						Field: "server_id",
						Value: service.ServerId,
					},
				})
				if err1 != nil {
					var pqErr *pq.Error
					if errors.As(err1, &pqErr) && pqErr.Code == "42P01" {
						err1 = nil
					}
				}
				if err1 != nil {
					return err1
				}
				// 删除服务
				err1 = serviceModel.Delete(ctx, service.ServerId)
				if err1 != nil {
					return err1
				}
				return nil
			})
			if err != nil {
				continue
			}
			num++
		}
	}
	resp = &types.BaseResp{
		Code: 0,
		Data: types.D{
			"total": total,
			"num":   num,
		},
		Message: "删除成功",
	}
	return
}
