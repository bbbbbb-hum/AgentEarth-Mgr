package source

import (
	"context"
	"errors"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/models/external"

	"github.com/zeromicro/go-zero/core/logx"
)

type AccountSyncTestToProdLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAccountSyncTestToProdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountSyncTestToProdLogic {
	return &AccountSyncTestToProdLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AccountSyncTestToProd syncs ExternalMcpServicesAccount rows by ids from current DB (test) to ProdDB.
func (l *AccountSyncTestToProdLogic) AccountSyncTestToProd(req *types.IdsReq) (resp *types.BaseResp, err error) {
	if l.svcCtx.ProdExternalMpcServicesAccountModel == nil {
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
		row, e := l.svcCtx.ExternalMpcServicesAccountModel.FindOne(l.ctx, id)
		if e != nil {
			if errors.Is(e, external.ErrNotFound) {
				missingIds = append(missingIds, id)
				continue
			}
			err = e
			return
		}

		e = l.svcCtx.ProdExternalMpcServicesAccountModel.UpsertWithId(l.ctx, row)
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


