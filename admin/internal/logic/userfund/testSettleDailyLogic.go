package userfund

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TestSettleDailyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTestSettleDailyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TestSettleDailyLogic {
	return &TestSettleDailyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TestSettleDailyLogic) TestSettleDaily(req *types.TestSettleReq) (resp *types.TestResp, err error) {
	targetDate := time.Now().AddDate(0, 0, -1) // 默认昨天
	if req.Date != "" {
		parsedDate, err := time.Parse("2006-01-02", req.Date)
		if err == nil {
			targetDate = parsedDate
		}
	}

	l.Logger.Infof("[Test] Triggering manual daily settlement for %s", targetDate.Format("2006-01-02"))

	logic := NewSettlementLogic(l.ctx, l.svcCtx)
	if err := logic.SettleAllUsersConsumption(targetDate); err != nil {
		return &types.TestResp{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &types.TestResp{
		Success: true,
		Message: "Settlement triggered successfully, check logs",
	}, nil
}
