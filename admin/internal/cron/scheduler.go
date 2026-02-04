package cron

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/logic/userfund"
	"AgentEarth-Mgr/admin/internal/svc"

	"github.com/robfig/cron/v3" //Go最常用的定时任务库
	"github.com/zeromicro/go-zero/core/logx"
)

type JobScheduler struct {
	cron   *cron.Cron          //指针，指向cron库的核心对象，用来管理所有的定时任务
	svcCtx *svc.ServiceContext //go-sero框架的服务上下文，依赖注入
	ctx    context.Context     //用户控制生命周期的上下文
}

func NewJobScheduler(ctx context.Context, svcCtx *svc.ServiceContext) *JobScheduler {
	// 默认时区为本地时间
	c := cron.New() //创建一个定时任务管理器
	return &JobScheduler{
		cron:   c,
		svcCtx: svcCtx,
		ctx:    ctx,
	}
}

func (s *JobScheduler) Start() {
	// 1. 注册日结任务 (每天凌晨 01:00 执行，结算前一天的数据)
	_, err := s.cron.AddFunc("0 1 * * *", func() {
		logx.Info("[Cron] 核销用户消费资金定时任务，开始执行...")
		logic := userfund.NewSettlementLogic(context.Background(), s.svcCtx)

		// 结算“昨天”的数据
		yesterday := time.Now().AddDate(0, 0, -1)
		if err := logic.SettleAllUsersConsumption(yesterday); err != nil {
			logx.Errorf("[Cron] 资金核销定时任务出现错误: %v", err)
		}
	})
	if err != nil {
		logx.Errorf("Failed to add settlement job: %v", err)
	}

	// 2. 注册过期清理任务 (每天凌晨 02:00 执行，执行过期扣减操作)
	_, err = s.cron.AddFunc("0 2 * * *", func() {
		logx.Info("[Cron] 过期扣减定时任务开始执行...")
		logic := userfund.NewExpirationLogic(context.Background(), s.svcCtx)

		if err := logic.ProcessExpirationLogic(time.Now()); err != nil {
			logx.Errorf("[Cron] 过期扣减定时任务执行失败: %v", err)
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
