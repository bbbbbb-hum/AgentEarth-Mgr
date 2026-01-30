package source

import (
	"context"
	"errors"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DetailLogic {
	return &DetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DetailLogic) Detail(req *types.DetailReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	detail, err := l.svcCtx.ExternalMcpServicesModel.FindOne(l.ctx, req.Id)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return
	}
	if detail == nil {
		err = errors.New("该数据不存在~")
		return
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"detail": detail,
		},
	}
	return
}
