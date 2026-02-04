package userfund

import (
	"context"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/models/users"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SettlementLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSettlementLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettlementLogic {
	return &SettlementLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SettlementStats 单条日消费核销的统计，用于汇总日志
type SettlementStats struct {
	WasRepeated   bool  // 该条日消费已有核销记录却仍进入流程（疑似重复执行，需排查）
	NewAllocCount int64 // 本次实际新增的核销记录条数
}

// ProcessUserDailyConsumption 核销用户每日消费记录的定时任务逻辑
// 逻辑说明:
// 1. 获取用户当日总消费金额
// 2. 查找用户所有有效充值记录(正值、未过期)，按FEFO原则(先过期先扣)排序
// 3. 逐笔计算实时余额并进行扣减，生成核销记录
// 4. 支持事务，确保资金操作的原子性
// 返回: 统计信息（是否异常重复进入、本次新增核销条数）、error
func (l *SettlementLogic) ProcessUserDailyConsumption(dailyRecord *users.AeUserConsumptionRecordDaily) (stats SettlementStats, err error) {
	// 将 float64 转换为 decimal 方便计算
	//1.定义还要扣多少钱
	amountToDeduct := decimal.NewFromFloat(dailyRecord.XlcreditConsume)

	// 如果没有消费，直接返回，如果当前用户今天没有消费记录，直接返回，不扣减
	if amountToDeduct.LessThanOrEqual(decimal.Zero) {
		return stats, nil
	}

	// 解析用户名用于日志展示（查不到则用 user_id 兜底）
	userDisplayName := dailyRecord.UserId
	if u, findErr := l.svcCtx.McpUserModel.FindOneByUserId(l.ctx, dailyRecord.UserId); findErr == nil && u.Username != "" {
		userDisplayName = u.Username
	}

	// 核销前检查：若该条日消费已在资金核销表中被完全分摊，则不再核销，避免短时间重复触发造成失误
	var allocatedSumStr string
	if errQuery := l.svcCtx.DB.QueryRowCtx(l.ctx, &allocatedSumStr, "SELECT COALESCE(SUM(deducted_amount), 0) FROM ae_recharge_allocation WHERE consumption_daily_id = $1", dailyRecord.Id); errQuery == nil {
		if allocatedSum, parseErr := decimal.NewFromString(allocatedSumStr); parseErr == nil && allocatedSum.GreaterThanOrEqual(amountToDeduct) {
			l.Logger.Infof("[ProcessUserDailyConsumption] 日消费ID=%d 用户=%s 已成功分摊（已核销总额 %s >= 消费金额 %s），不再核销", dailyRecord.Id, userDisplayName, allocatedSum.String(), amountToDeduct.String())
			return stats, nil
		}
	}

	// 开启事务，l.svcCtx.DB.TransactCtx 相当于 @Transactional 注解
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		// 0. 异常检测：正常应每天只执行一次，若该日消费已有核销记录却仍进入流程，说明可能重复触发或定时任务异常
		var existingCount int64
		_ = session.QueryRowCtx(ctx, &existingCount, "SELECT COUNT(*) FROM ae_recharge_allocation WHERE consumption_daily_id = $1", dailyRecord.Id)
		if existingCount > 0 {
			stats.WasRepeated = true
			l.Logger.Errorf("[ProcessUserDailyConsumption] 异常：用户 %s 日消费ID=%d 已有 %d 条核销记录却仍进入核销流程，疑似定时任务重复执行或配置异常，请排查。本次将仅做幂等处理（不重复插入）", userDisplayName, dailyRecord.Id, existingCount)
		}

		// 1. 查找候选记录
		// 条件: user_id 匹配, xlcredit_amount > 0, expire_time > NOW() (或为空/永久有效)
		// 排序: expire_time ASC (NULLS LAST), pay_time ASC
		// 注意: Postgres中 NULL 比较特殊，这里假设 expire_time 为 NULL 是永久有效，应该放在最后扣?
		// 或者是根据需求: "expire_time > NOW()" 已经排除了已过期的。
		// 通常逻辑: 有过期时间的优先扣，没过期时间的(永久)最后扣。
		// SQL: expire_time IS NOT NULL 先排, 然后按时间升序。
		// 你的需求: "expire_time ASC" -> 在 Postgres 中 NULL 默认是最大的，所以 NULL 会排在最后，符合"永久有效最后扣"的逻辑。

		query := `
			SELECT id, user_id, xlcredit_amount, pay_time, create_time, update_time, charge_source, charge_type, remark, operator, expire_time, related_recharge_id
			FROM ae_user_recharge_record
			WHERE user_id = $1 
			  AND xlcredit_amount > 0 
			  AND (expire_time IS NULL OR expire_time > $2)
			ORDER BY expire_time ASC, pay_time ASC
		`
		//查所有的候选充值余额大于0，且未过期，按过期时间排序，先过期先扣
		var candidates []users.AeUserRechargeRecord                                          //用来接收查询结果
		err := session.QueryRowsCtx(ctx, &candidates, query, dailyRecord.UserId, time.Now()) //后面两个参数是给sql语句传参
		if err != nil {
			return err
		}

		if len(candidates) == 0 {
			l.Logger.Infof("[ProcessUserDailyConsumption] 用户 %s 没有任何可用的充值记录来扣减 %v", userDisplayName, amountToDeduct)
			// No return here, let it fall through to the insufficient balance check
		}

		// 2. 循环扣减，如果还要扣的钱小于0，则跳出循环
		for _, record := range candidates {
			if amountToDeduct.LessThanOrEqual(decimal.Zero) {
				break
			}

			// --- 现算余额 ---
			// 复用 balance_helper.go 中的 CalculateRealTimeBalance 方法
			// 该方法已支持 sqlx.Session (通过 Queryable 接口)
			// 注意: 虽然 Model 中 XlcreditAmount 是 int64，但数据库是 numeric(20,8)。
			// 这里将其转为 decimal 进行高精度计算。
			initialAmount := decimal.NewFromFloat(record.XlcreditAmount)

			balance, err := CalculateRealTimeBalance(ctx, session, record.Id, initialAmount)
			if err != nil {
				l.Logger.Errorf("Failed to calc balance for record %d: %v", record.Id, err)
				return err // 计算余额失败，终止事务
			}

			if balance.LessThanOrEqual(decimal.Zero) {
				continue // 空桶，跳过
			}

			// --- 决策本轮扣减额 ---
			var actualDeduct decimal.Decimal
			if balance.GreaterThanOrEqual(amountToDeduct) {
				actualDeduct = amountToDeduct //够扣，把要扣的全扣掉就好
			} else {
				actualDeduct = balance //不够扣，扣掉当前余额
			}

			// --- 落库：插入核销记录 ---
			// 对应 ae_recharge_allocation 表
			// 使用 ON CONFLICT DO NOTHING 避免重复核销
			insertAllocQuery := `
				INSERT INTO ae_recharge_allocation 
				(consumption_daily_id, recharge_record_id, deducted_amount, create_time)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (consumption_daily_id, recharge_record_id) DO NOTHING
			`
			result, err := session.ExecCtx(ctx, insertAllocQuery, dailyRecord.Id, record.Id, actualDeduct, time.Now())
			if err != nil {
				l.Logger.Errorf("Failed to insert allocation for record %d: %v", record.Id, err)
				return err
			}

			// 检查是否实际插入了数据
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if rowsAffected == 0 {
				l.Logger.Infof("[ProcessUserDailyConsumption] 幂等跳过: 日消费ID=%d 充值批次ID=%d 已存在核销记录", dailyRecord.Id, record.Id)
				// 已经结算过，不应该重复扣减 amountToDeduct，也不应该记日志说又扣了一次
				// 但这里因为 amountToDeduct 是循环变量，如果不扣减，会导致死循环或者重复尝试下一条
				// 正确的逻辑是：如果发现已经结算过，说明这笔消费的这部分金额已经被处理了。
				// 我们应该假设这部分金额已经从 amountToDeduct 中扣除了（因为之前已经成功了）。
				amountToDeduct = amountToDeduct.Sub(actualDeduct)
				continue
			}
			stats.NewAllocCount++

			// --- 日志记录 ---
			// 记录本次扣减的详细信息：用户、金额、充值批次ID、过期时间、操作时间
			expireTimeStr := "永久有效"
			if record.ExpireTime.Valid {
				expireTimeStr = record.ExpireTime.Time.Format(time.RFC3339)
			}
			l.Logger.Infof("[ProcessUserDailyConsumption] 核销详情: 用户=%s, 扣减金额=%s, 充值批次ID=%d, 批次过期时间=%s, 操作时间=%s, 剩余需扣=%s",
				userDisplayName, actualDeduct.String(), record.Id, expireTimeStr, time.Now().Format(time.RFC3339), amountToDeduct.Sub(actualDeduct).String())

			// --- 更新 amountToDeduct ---
			amountToDeduct = amountToDeduct.Sub(actualDeduct)
		}

		// 3. 兜底检查
		if amountToDeduct.GreaterThan(decimal.Zero) {
			l.Logger.Errorf("[ProcessUserDailyConsumption] 用户 %s 日消费 %v 余额不足，剩余待扣: %v",
				userDisplayName, dailyRecord.XlcreditConsume, amountToDeduct)
			// 这里不返回 error，允许事务提交，因为已经扣减的部分是有效的。
			// 剩余未扣减的部分构成了"坏账"或"透支"，需要在后续流程处理或通知管理员。
		}

		return nil
	})
	return stats, err
}

// SettleAllUsersConsumption 处理指定日期所有用户的消费结算 (Cron任务入口)
func (l *SettlementLogic) SettleAllUsersConsumption(targetDate time.Time) error {
	// 1. 查询当天所有有消费记录的用户数据
	query := `
		SELECT id, user_id, day, xlcredit_consume, create_time
		FROM ae_user_consumption_record_daily
		WHERE day = $1 AND xlcredit_consume > 0
	`

	// 注意：targetDate 应该被截断为“天”，确保匹配数据库的 date 类型
	dateStr := targetDate.Format("2006-01-02")

	var dailyRecords []users.AeUserConsumptionRecordDaily
	err := l.svcCtx.DB.QueryRowsCtx(l.ctx, &dailyRecords, query, dateStr)
	if err != nil {
		l.Logger.Errorf("[SettleAllUsers] Failed to query daily records for %s: %v", dateStr, err)
		return err
	}

	if len(dailyRecords) == 0 {
		l.Logger.Infof("[SettleAllUsers] 日期 %s 未发现任何消费记录", dateStr)
		return nil
	}

	l.Logger.Infof("[SettleAllUsers] 开始核销日期 %s 的消费记录，共 %d 条用户日消费（已完全分摊的将跳过；若异常重复进入则仅做幂等处理）", dateStr, len(dailyRecords))

	// 2. 遍历处理 (遇到错误记录日志但不中断循环)，并汇总统计
	var repeatCount int
	var totalNewAllocs int64
	for _, record := range dailyRecords {
		stats, procErr := l.ProcessUserDailyConsumption(&record)
		if procErr != nil {
			l.Logger.Errorf("[SettleAllUsers] Failed to settle for user %s: %v", record.UserId, procErr)
			continue
		}
		if stats.WasRepeated {
			repeatCount++
		}
		totalNewAllocs += stats.NewAllocCount
	}

	l.Logger.Infof("[SettleAllUsers] 日期 %s 核销完成：共 %d 条日消费，其中 %d 条已有核销记录（疑似重复执行，请排查），实际新增 %d 条核销", dateStr, len(dailyRecords), repeatCount, totalNewAllocs)
	return nil
}
