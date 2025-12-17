package source

import (
	"AgentEarth-Mgr/models"
	configModel "AgentEarth-Mgr/models/config"
	"context"
	"errors"
	"fmt"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type AccountSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAccountSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountSyncLogic {
	return &AccountSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AccountSyncLogic) AccountSync(req *types.AccountSyncReq) (resp *types.BaseResp, err error) {
	var conditions []models.Condition
	if len(req.Ids) > 0 {
		conditions = append(conditions, models.Condition{
			Field:  "id",
			Symbol: "IN",
			Value:  req.Ids,
		})
	}
	list, total, err := l.svcCtx.ExternalMpcServicesAccountModel.GetList(l.ctx, models.ListConditions{
		Conditions: conditions,
		Pages:      models.Pages{},
	}, true)
	if err != nil {
		return
	}
	var syncNum int
	if total > 0 {
		for _, account := range list {
			err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
				sessConn := sqlx.NewSqlConnFromSession(session)
				serviceConfigModel := configModel.NewAeMcpExternalServicesConfigModel(sessConn)
				serviceConfigAccountModel := configModel.NewAeMcpExternalServicesAccountModel(sessConn)
				serviceConfig, err1 := serviceConfigModel.FindOneByCondition(ctx, []models.Condition{
					{
						Field:  "name",
						Symbol: "=",
						Value:  account.Name,
					},
				})
				if err1 != nil && !errors.Is(err1, sqlx.ErrNotFound) {
					return err1
				}
				if serviceConfig == nil {
					return fmt.Errorf("未找到对应的服务配置:%")
				}
				var serviceConfigAccount = &configModel.AeMcpExternalServicesAccount{
					Name:       account.Name,
					AuthInfo:   account.AuthInfo,
					ConfigId:   serviceConfig.Id,
					CreateTime: time.Now(),
					UpdateTime: time.Now(),
					Status:     "unused",
				}
				_, err2 := serviceConfigAccountModel.Insert(ctx, serviceConfigAccount)
				if err2 != nil {
					return err2
				}
				return nil
			})
			if err != nil {
				l.Errorf("同步账号失败,err:%s,account:%s", err, account.Name)
			} else {
				syncNum++
			}
		}
	}
	resp = &types.BaseResp{
		Code: 0,
		Data: types.D{
			"total":   total,
			"syncNum": syncNum,
		},
		Message: "success",
	}
	return
}
