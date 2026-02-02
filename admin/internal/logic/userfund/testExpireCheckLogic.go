package userfund

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestExpireCheckLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTestExpireCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestExpireCheckLogic {
	return &TestExpireCheckLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TestExpireCheckLogic) TestExpireCheck() (resp *types.TestResp, err error) {
	l.Logger.Infof("[Test] Triggering manual expiration check")

	logic := NewExpirationLogic(l.ctx, l.svcCtx)
	if err := logic.ProcessExpirationLogic(time.Now()); err != nil {
		return &types.TestResp{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &types.TestResp{
		Success: true,
		Message: "过期清理任务已触发，请查看日志（含每笔扣减和汇总）",
	}, nil
}
