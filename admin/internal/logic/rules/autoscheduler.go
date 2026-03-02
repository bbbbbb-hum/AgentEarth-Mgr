package rules

import (
	"context"
	"strings"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"

	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logx"
)

// 83240611是自己约定的一个分布式“锁名称”
// pgsql的“建议锁(Advisory Lock)机制”，允许我们用一个整数(或一对整数)给锁命名
// 这个数值不需要插入到任何表里，只是传给pgsql服务器内部的一个锁管理模块
const scheduleAdvisoryLockKey int64 = 83240611

// 一个全局的cron解析器
var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

// calcNextRunTime负责根据cron表达式和当前时间算出“下一次”执行时间
func calcNextRunTime(expr string, base time.Time) (time.Time, error) {
	// 用全局cronParser解析表达式，先TrimSpace去掉两边空白
	s, err := cronParser.Parse(strings.TrimSpace(expr))
	if err != nil {
		return time.Time{}, err
	}
	//把base时间转换到本地时区，并且Truncate到分钟精度
	base = base.In(time.Local).Truncate(time.Minute)
	// s.Next(base) 会给出“base 之后”第一次满足 cron 表达式的时间。
	return s.Next(base), nil
}

// StartAutoRuleScheduler 启动每分钟扫描一次的自动规则调度器。
func StartAutoRuleScheduler(ctx context.Context, svcCtx *svc.ServiceContext) {
	//启动一个新的goroutine，这样不会阻塞调用方
	//定时任务用一个新的goroutine在后台独立的跑，主逻辑继续执行，不会影响HTTP服务启动
	go func() {
		// 创建一个每分钟触发一次的 Ticker
		// Ticker就是一个“定时器通道”：每过一段时间往 channal 里发一个当前时间
		// Ticker是一个按固定时间间隔"滴答"一次的计时器，内部有一个channal：ticker.c,类型是<-chan time.Time
		ticker := time.NewTicker(time.Minute)
		// 函数退出时停止ticker，释放资源
		defer ticker.Stop()

		//定义真正执行调度逻辑的闭包函数run
		run := func() {
			//调度 runAutoRulesOnce 执行一次扫描 + 执行
			if err := runAutoRulesOnce(ctx, svcCtx); err != nil {
				//如果发生错误，打印日志，不要报错停止服务，让后续轮询还可以继续执行
				logx.WithContext(ctx).Errorf("自动规则调度执行失败: %v", err)
			}
		}

		// 先立即把所有已经到时间的规则执行一次 (服务刚启动时不等待下一分钟，立即开始扫描)
		run()
		//进入一个长期运行的循环，在这个goroutine里一直等事件
		for {
			select {
			case <-ctx.Done(): // 如果上层传入的 ctx 被取消（比如服务关闭），就退出循环，结束 goroutine
				return
			case <-ticker.C: // 等待闹钟响，每到一个时间点（每分钟一次）就再跑一次调度。
				run()
			}
		}
	}()
}

// runAutoRulesOnce 代表 “执行一次完整的调度循环”
//  1. 争抢 advisory lock（保证多实例下只有一个在跑真正逻辑）；
//  2. 查出所有需要执行的规则；
//  3. 对每条规则调用 ExecuteRule；
//  4. 更新规则的 last_run_time、next_run_time。
func runAutoRulesOnce(ctx context.Context, svcCtx *svc.ServiceContext) error {
	// 先尝试获取 PostgreSQL advisory lock，防止多实例并发执行同一套调度。
	locked, err := svcCtx.RuleQueryModel.TryAdvisoryLock(ctx, scheduleAdvisoryLockKey)
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}

	//函数结束时释放 advisory lock (defer 确保不管中途return还是panic都会执行)
	defer func() { _ = svcCtx.RuleQueryModel.AdvisoryUnlock(ctx, scheduleAdvisoryLockKey) }()

	// 查出所有“到期需要执行的规则”的 id
	ids, err := svcCtx.RuleQueryModel.ListDueRuleIds(ctx, 200)
	if err != nil {
		return err
	}

	for _, id := range ids {
		rule, findErr := svcCtx.RuleModel.FindOne(ctx, id)
		if findErr != nil {
			logx.WithContext(ctx).Errorf("查询规则失败, id=%d, err=%v", id, findErr)
			continue
		}

		//虽然sql里已经筛选过status = 'active'，这里再做一次防御性校验
		if rule.Status != "active" {
			continue // 状态不是active就直接跳过
		}

		//execAt 记录这次执行的时间点，用于更新last_run_time和计算next_run_time的基准
		execAt := time.Now()
		//调用 ExecuteRule 真正执行规则逻辑
		//execSource = “cron”， operator = “cron”(表示是定时任务触发)
		_, execErr := ExecuteRule(ctx, svcCtx, rule, "cron", "cron")
		if execErr != nil {
			logx.WithContext(ctx).Errorf("执行规则失败, id=%d, err=%v", rule.Id, execErr)
			//出错时不return，继续处理下一条规则，避免一条挂了影响所有
			continue
		}
		// 规则执行完后，需要算出下一次执行时间。
		nextRun, nextErr := calcNextRunTime(rule.TriggerKey, execAt)
		if nextErr != nil {
			logx.WithContext(ctx).Errorf("计算下次执行时间失败, id=%d, cron=%s, err=%v", rule.Id, rule.TriggerKey, nextErr)
			continue
		}
		// 更新 ae_rule 表的 last_run_time、next_run_time、update_time
		if updErr := svcCtx.RuleQueryModel.UpdateRuleRunTimes(ctx, rule.Id, execAt, nextRun, time.Now()); updErr != nil {
			logx.WithContext(ctx).Errorf("更新规则下次执行时间失败, id=%d, err=%v", rule.Id, updErr)
		} else {
			logx.WithContext(ctx).Infof("自动规则 %d 执行成功，下次执行时间: %v", rule.Id, nextRun)
		}
	}

	return nil
}
