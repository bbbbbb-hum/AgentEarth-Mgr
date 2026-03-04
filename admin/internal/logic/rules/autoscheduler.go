package rules

import (
	"context"
	"strings"
	"sync"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"

	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logx"
)

// 83240611是自己约定的一个分布式“锁名称”
// pgsql的“建议锁(Advisory Lock)机制”，允许我们用一个整数(或一对整数)给锁命名
// 这个数值不需要插入到任何表里，只是传给pgsql服务器内部的一个锁管理模块
const scheduleAdvisoryLockKey int64 = 83240611

// autoRuleScheduler 负责把规则的 cron 表达式注册到 robfig/cron 中，由 cron 按表达式触发。
type autoRuleScheduler struct {
	baseCtx context.Context
	svcCtx  *svc.ServiceContext

	cron    *cron.Cron
	mu      sync.Mutex
	entries map[int64]cron.EntryID
}

// 包级全局指针变量，保证全局只有一个调度器，避免重复启动cron
var globalAutoRuleScheduler *autoRuleScheduler

// StartAutoRuleScheduler 初始化自动规则调度器：争抢一次分布式锁，加载所有已启用规则并注册到 cron。
// 之后的定时触发完全由 robfig/cron 接管，不再轮询。
func StartAutoRuleScheduler(ctx context.Context, svcCtx *svc.ServiceContext) {
	logger := logx.WithContext(ctx)

	// 防御性检查：如果已经有全局调度器了，就不要重复初始化
	if globalAutoRuleScheduler != nil {
		return
	}

	// 多实例下，通过 advisory lock 选出唯一“调度者”实例
	locked, err := svcCtx.RuleQueryModel.TryAdvisoryLock(ctx, scheduleAdvisoryLockKey)
	if err != nil {
		logger.Errorf("自动规则调度器尝试获取分布式锁失败: %v", err)
		return
	}
	if !locked {
		logger.Infof("自动规则调度器分布式锁未获取到，当前实例不负责自动规则调度")
		return
	}
	//构造调度器实例，并挂到全局变量上
	scheduler := &autoRuleScheduler{
		baseCtx: ctx,
		svcCtx:  svcCtx,
		cron:    cron.New(),                   //使用默认解析规则的 cron 调度器
		entries: make(map[int64]cron.EntryID), //初始化map存储ruleID -> 对应的 cron entryID
	}
	globalAutoRuleScheduler = scheduler

	// 启动时从 DB 加载所有已启用规则，并注册到 cron
	if err := scheduler.loadAndRegisterAllRules(ctx); err != nil {
		logger.Errorf("自动规则调度器加载规则失败: %v", err)
	}

	// 启动调度器（内部会起自己的 goroutine），按表达式触发。
	// 这里 Start() 内部会起自己的 goroutine，按注册的 cron 表达式定时调用对应的函数
	scheduler.cron.Start()
	logger.Infof("自动规则调度器已启动")

	// 监听上层 ctx 关闭，关闭调度器并尝试释放 advisory lock。
	go func() {
		<-ctx.Done()
		logger.Info("自动规则调度器收到上层上下文取消信号，准备停止")
		scheduler.cron.Stop()
		if err := svcCtx.RuleQueryModel.AdvisoryUnlock(context.Background(), scheduleAdvisoryLockKey); err != nil {
			logger.Errorf("自动规则调度器释放分布式锁失败: %v", err)
		}
	}()
}

// loadAndRegisterAllRules 启动时加载所有已启用规则，并把 cron 表达式注册进调度器。
func (s *autoRuleScheduler) loadAndRegisterAllRules(ctx context.Context) error {
	// 从 models 层获取所有 active 规则的精简信息（id + cron_value + active）
	rows, err := s.svcCtx.RuleQueryModel.ListActiveRulesForScheduler(ctx)
	if err != nil {
		return err
	}

	// 逐条规则注册进 cron 调度器
	for _, r := range rows {
		s.registerOrUpdateRule(ctx, r.Id, r.CronValue)
	}

	return nil
}

// registerOrUpdateRule 将一条规则的 cron 表达式注册到调度器中；如果已存在则先移除再重新注册。
// 如果这条规则之前已经注册过定时任务，则先移除旧的 entry，再用新的表达式重新注册，避免一条规则被多次注册。
func (s *autoRuleScheduler) registerOrUpdateRule(ctx context.Context, ruleID int64, cronExpr string) {
	cronExpr = strings.TrimSpace(cronExpr)
	if cronExpr == "" {
		logx.WithContext(ctx).Errorf("自动规则调度器注册规则时发现 cron 表达式为空, rule_id=%d", ruleID)
		return
	}

	// 加锁，表示从这里开始，这个协程独占对共享数据的访问权,对s.entries的map操作，s.cron注册/移除entry等
	s.mu.Lock()
	defer s.mu.Unlock()

	// 如果这条规则之前已经在 cron 中注册过，先移除旧的定时任务。
	if entryID, ok := s.entries[ruleID]; ok {
		s.cron.Remove(entryID)
		delete(s.entries, ruleID)
	}

	// 为当前规则添加一个 cron 任务
	// 到达 cronExpr 对应的时间点时，调用 s.runRule(ruleID) 执行这条规则
	rid := ruleID //复制一份ruleID，在闭包匿名函数中固定住当前规则id，避免循环等导致变量混乱
	//核心：向s.cron定时任务调度器中注册定时任务，后面的fun(){}表示到点后执行的函数
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.runRule(rid)
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("自动规则调度器注册规则失败, rule_id=%d, cron=%s, err=%v", ruleID, cronExpr, err)
		return
	}

	s.entries[ruleID] = entryID
	logx.WithContext(ctx).Infof("自动规则调度器已注册规则, rule_id=%d, cron=%s", ruleID, cronExpr)
}

// unregisterRule 从调度器中移除某条规则的 cron 任务。
func (s *autoRuleScheduler) unregisterRule(ctx context.Context, ruleID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, ok := s.entries[ruleID]
	if !ok {
		return
	}

	s.cron.Remove(entryID)
	delete(s.entries, ruleID)
	logx.WithContext(ctx).Infof("自动规则调度器已移除规则, rule_id=%d", ruleID)
}

// runRule 是被 cron 调度器真正调用的函数：
// 每当某条规则的 cron 表达式触发时，就会执行一次 runRule(ruleID)
func (s *autoRuleScheduler) runRule(ruleID int64) {
	// 这里使用一个带超时的上下文，避免单次执行无限卡住。
	ctx, cancel := context.WithTimeout(s.baseCtx, 10*time.Minute)
	defer cancel()

	// 全局 panic 保护，防止某条规则执行时 panic 导致整个调度 goroutine 崩溃。
	defer func() {
		if r := recover(); r != nil {
			logx.WithContext(ctx).Errorf("自动规则调度执行 panic, rule_id=%d, err=%v", ruleID, r)
		}
	}()

	rule, err := s.svcCtx.RuleModel.FindOne(ctx, ruleID)
	if err != nil {
		logx.WithContext(ctx).Errorf("自动规则调度查询规则失败, rule_id=%d, err=%v", ruleID, err)
		return
	}
	if rule.Active != "active" {
		// 状态已变更为非 active，则不再执行。
		return
	}

	// 自动执行时将操作者标记为 "cron"，便于对账明细中区分来源。
	if _, execErr := ExecuteRule(ctx, s.svcCtx, rule, "cron", "cron"); execErr != nil {
		logx.WithContext(ctx).Errorf("自动规则调度执行规则失败, rule_id=%d, err=%v", ruleID, execErr)
		return
	}
	logx.WithContext(ctx).Infof("自动规则调度执行规则成功, rule_id=%d", ruleID)
}

// helper：供保存/启用/停用/删除规则的 logic 使用，把规则变更同步到调度器。
func registerRuleInScheduler(ctx context.Context, ruleID int64, cronExpr string) {
	if globalAutoRuleScheduler == nil {
		// 当前实例可能不是“调度者”（没拿到 advisory lock），直接跳过。
		return
	}
	globalAutoRuleScheduler.registerOrUpdateRule(ctx, ruleID, cronExpr)
}

// unregisterRuleFromScheduler 是给停用/删除规则时调用的辅助函数。
// 它会从调度器中移除该规则对应的 cron 任务
func unregisterRuleFromScheduler(ctx context.Context, ruleID int64) {
	if globalAutoRuleScheduler == nil {
		return
	}
	globalAutoRuleScheduler.unregisterRule(ctx, ruleID)
}
