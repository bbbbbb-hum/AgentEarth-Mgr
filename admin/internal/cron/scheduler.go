package cron

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/logic/userfund"
	"AgentEarth-Mgr/admin/internal/svc"

	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logx"
)

type JobScheduler struct {
	cron   *cron.Cron
	svcCtx *svc.ServiceContext
	ctx    context.Context
}

func NewJobScheduler(ctx context.Context, svcCtx *svc.ServiceContext) *JobScheduler {
	// 默认时区为本地时间
	c := cron.New()
	return &JobScheduler{
		cron:   c,
		svcCtx: svcCtx,
		ctx:    ctx,
	}
}

func (s *JobScheduler) Start() {
	// 1. 注册日结任务 (每天凌晨 01:00 执行，结算前一天的数据)
	_, err := s.cron.AddFunc("0 1 * * *", func() {
		logx.Info("[Cron] Starting daily settlement job...")
		logic := userfund.NewSettlementLogic(context.Background(), s.svcCtx)

		// 结算“昨天”的数据
		yesterday := time.Now().AddDate(0, 0, -1)
		if err := logic.SettleAllUsersConsumption(yesterday); err != nil {
			logx.Errorf("[Cron] Daily settlement failed: %v", err)
		}
	})
	if err != nil {
		logx.Errorf("Failed to add settlement job: %v", err)
	}

	// 2. 注册过期清理任务 (每天凌晨 02:00 执行，执行过期扣减操作)
	_, err = s.cron.AddFunc("0 2 * * *", func() {
		logx.Info("[Cron] Starting expiration check job...")
		logic := userfund.NewExpirationLogic(context.Background(), s.svcCtx)

		if err := logic.ProcessExpirationLogic(time.Now()); err != nil {
			logx.Errorf("[Cron] Expiration check failed: %v", err)
		}
	})
	if err != nil {
		logx.Errorf("Failed to add expiration job: %v", err)
	}

	s.cron.Start()
	logx.Info("[Cron] Scheduler started")
}

func (s *JobScheduler) Stop() {
	s.cron.Stop()
}
