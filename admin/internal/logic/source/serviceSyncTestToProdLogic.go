package source

import (
	"context"
	"errors"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/models/external"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceSyncTestToProdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceSyncTestToProdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceSyncTestToProdLogic {
	return &ServiceSyncTestToProdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ServiceSyncTestToProd syncs ExternalMcpServices rows by ids from test DB to prod DB (upsert by id).
func (l *ServiceSyncTestToProdLogic) ServiceSyncTestToProd(req *types.IdsReq) (resp *types.BaseResp, err error) {
	if l.svcCtx.ProdExternalMcpServicesModel == nil {
		err = errors.New("ProdDB未配置：请在配置文件中设置 ProdDB.DataSource")
		return
	}
	if len(req.Ids) == 0 {
		err = errors.New("ids不能为空")
		return
	}
	if len(req.Ids) > 50 {
		err = errors.New("一次最多同步50条")
		return
	}

	var (
		synced     int64
		missingIds []int64
	)

	for _, id := range req.Ids {
		// Source: current env DB (typically test). Target: ProdDB.
		row, e := l.svcCtx.ExternalMcpServicesModel.FindOne(l.ctx, id)
		if e != nil {
			if errors.Is(e, external.ErrNotFound) {
				missingIds = append(missingIds, id)
				continue
			}
			err = e
			return
		}

		e = l.svcCtx.ProdExternalMcpServicesModel.UpsertWithId(l.ctx, row)
		if e != nil {
			err = e
			return
		}
		synced++
	}

	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"synced":      synced,
			"missing_ids": missingIds,
		},
	}
	return
}


