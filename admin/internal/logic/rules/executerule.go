package rules

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	ruleModel "AgentEarth-Mgr/models/rules"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var errIdempotentConflict = errors.New("idempotent conflict")

// ExecuteRule 执行单条规则，供手动和自动调度共用。
func ExecuteRule(ctx context.Context, svcCtx *svc.ServiceContext, rule *ruleModel.AeRule, execSource, operator string) (int, error) {
	logger := logx.WithContext(ctx)

	// 1. 定义用于反序列化 FilterConfig 的 struct（已不再包含用户状态筛选）。
	var fCfg struct {
		MinRegDays          int     `json:"min_reg_days"`
		MaxRegDays          int64   `json:"max_reg_days"`
		LastLoginWithinDays int64   `json:"last_login_within_days"`
		MinLastMonthConsume float64 `json:"min_last_month_consume"`
		MaxLastMonthConsume float64 `json:"max_last_month_consume"`
		MinBalance          float64 `json:"min_balance"`
		MaxBalance          float64 `json:"max_balance"`
		RegChannel          string  `json:"reg_channel"`
		MinHistoryRecharge  float64 `json:"min_history_recharge"`
		MaxHistoryRecharge  float64 `json:"max_history_recharge"`
	}

	// 2. 定义用于反序列化 ActionConfig 的 struct。
	var aCfg struct {
		Type           string  `json:"type"`
		Amount         float64 `json:"amount"`
		ExpireStrategy string  `json:"expire_strategy"` //过期策略
	}
	//3.解析规则上的JSON配置
	if err := json.Unmarshal([]byte(rule.FilterConfig), &fCfg); err != nil {
		return 0, fmt.Errorf("解析筛选配置失败: %w", err)
	}
	if err := json.Unmarshal([]byte(rule.ActionConfig), &aCfg); err != nil {
		return 0, fmt.Errorf("解析动作配置失败: %w", err)
	}
	// 兼容旧数据：历史规则未设置余额区间时通常是 0/0，语义应为“不限制”。
	if fCfg.MinBalance == 0 && fCfg.MaxBalance == 0 {
		fCfg.MinBalance = -1
		fCfg.MaxBalance = -1
	}
	if aCfg.Type == "" {
		aCfg.Type = "add_points"
	}
	//将动作类型和金额转换为真正往充值表里添加的数据
	rechargeAmount, chargeType, normalizedActionType, err := resolveRechargeAction(aCfg.Type, aCfg.Amount)
	if err != nil {
		return 0, err
	}
	aCfg.Type = normalizedActionType
	if fCfg.MaxLastMonthConsume > 0 && fCfg.MinLastMonthConsume > 0 && fCfg.MaxLastMonthConsume < fCfg.MinLastMonthConsume {
		return 0, fmt.Errorf("筛选条件无效: max_last_month_consume 小于 min_last_month_consume")
	}
	if fCfg.MaxBalance >= 0 && fCfg.MinBalance >= 0 && fCfg.MaxBalance < fCfg.MinBalance {
		return 0, fmt.Errorf("筛选条件无效: max_balance 小于 min_balance")
	}

	filter := ruleModel.AudienceFilter{
		MinRegDays:          fCfg.MinRegDays,
		MaxRegDays:          fCfg.MaxRegDays,
		LastLoginWithinDays: fCfg.LastLoginWithinDays,
		MinLastMonthConsume: fCfg.MinLastMonthConsume,
		MaxLastMonthConsume: fCfg.MaxLastMonthConsume,
		MinBalance:          fCfg.MinBalance,
		MaxBalance:          fCfg.MaxBalance,
		RegChannel:          fCfg.RegChannel,
		MinHistoryRecharge:  fCfg.MinHistoryRecharge,
		MaxHistoryRecharge:  fCfg.MaxHistoryRecharge,
	}

	// 根据筛选条件查出“要执行动作的所有 user_id 列表”
	userIds, err := svcCtx.RuleQueryModel.ListAudienceUserIds(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("查询目标用户失败: %w", err)
	}
	// 如果没有用户被命中，就直接返回 0，不算错误。
	if len(userIds) == 0 {
		logger.Infof("规则无命中用户, rule=%d, source=%s", rule.Id, execSource)
		return 0, nil
	}

	// 9. 处理执行过程中的时间/幂等/备注等。
	now := time.Now()
	// 根据过期策略算出本次加积分的过期时间
	expireTime := buildExpireTime(aCfg.ExpireStrategy, now, rechargeAmount)

	// slotTime 用于确定这次执行“属于哪个时间窗口”，和 periodKey 组合出幂等键。
	slotTime := now
	// 如果是 cron 自动执行，且规则有 next_run_time（下次执行时间），
	// 那么我们用 next_run_time 作为这次执行的时间槽，更符合“计划时间”。
	if execSource == "cron" && rule.NextRunTime.Valid {
		slotTime = rule.NextRunTime.Time
	}

	// periodKey：触发时间戳的字符串，用于幂等键。同一规则+同一用户+同一触发时间只执行一次。
	// cron：精确到分钟 200601021504（年月日时分）
	// manual：精确到秒 20060102150405，便于同分钟内多次手动触发分别记一次
	periodKey := slotTime.In(time.Local).Format("200601021504")
	if execSource == "manual" {
		periodKey = slotTime.In(time.Local).Format("20060102150405")
	}
	// remark：给执行日志的“来源说明”。
	remark := "定时任务触发"
	if execSource == "manual" {
		remark = "管理员手动触发"
	}

	// 10. 针对每个用户依次执行“加点数 + 写执行日志”，二者放在同一个事务里。
	successCount := 0
	for _, uid := range userIds {
		// 幂等键 = rule_id + user_id + 触发时间戳（无 execSource，同一规则同用户同时间只执行一次）
		idemKey := fmt.Sprintf("rule_%d_user_%s_%s", rule.Id, uid, periodKey)
		// 插入到充值记录表里的备注
		rechargeRemark := fmt.Sprintf("规则执行充值 | 规则ID:%d | 触发:%s", rule.Id, remark)

		// 对于每个用户，以事务的方式执行“加钱 + 写日志”：
		//  - 用 svcCtx.DB.TransactCtx 开启事务，内部通过 WithSession 绑定同一个 session。
		//  - 任一操作失败（或幂等唯一键冲突），都会导致整个事务回滚。
		txErr := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
			// 事务内统一使用基于 session 的 model
			txRechargeModel := svcCtx.UserRechargeRecordModel.WithSession(session)
			txLogModel := ruleModel.NewAeRuleExecutionLogModel(sqlx.NewSqlConnFromSession(session))

			// 10.1 幂等预检查：同一幂等键如果已经存在执行日志，则直接跳过，不再加钱。
			existed, findErr := txLogModel.FindOneByIdempotentKey(txCtx, idemKey)
			if findErr != nil && findErr != ruleModel.ErrNotFound {
				return fmt.Errorf("查询执行日志幂等键失败: %w", findErr)
			}
			if existed != nil {
				logger.Infof("幂等性拦截，已存在执行日志，跳过重复执行: %s", idemKey)
				return nil
			}

			// 10.2 插入充值记录（必须带过期时间，如果失败则直接报错，不做降级）
			rechargeRecordID, rechargeErr := txRechargeModel.InsertRechargeRecordWithExpire(
				txCtx,
				uid,
				rechargeAmount,
				now,
				now,
				now,
				chargeType,
				rechargeRemark,
				operator,
				expireTime,
			)
			if rechargeErr != nil {
				return fmt.Errorf("插入充值记录失败: %w", rechargeErr)
			}

			// 10.3 插入执行日志，携带幂等键；如唯一索引冲突，则整笔事务回滚，实现“加钱 + 日志”一起幂等。
			detailRemark := fmt.Sprintf("%s | 规则ID:%d | 金额:%.2f | 充值记录:%d", remark, rule.Id, rechargeAmount, rechargeRecordID)
			logRow := &ruleModel.AeRuleExecutionLog{
				RuleId:        rule.Id,
				UserId:        uid,
				ActionType:    aCfg.Type,
				ChangeAmount:  rechargeAmount,
				Status:        "success",
				Remark:        sql.NullString{String: detailRemark, Valid: true},
				ExecSource:    execSource,
				Operator:      operator,
				IdempotentKey: idemKey,
				CreatedTime:   now,
				UpdateTime:    now,
			}
			if _, err := txLogModel.Insert(txCtx, logRow); err != nil {
				if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
					// 触发 idempotent_key 唯一约束，认为是并发/重试导致的重复执行，整笔事务回滚，不产生副作用。
					logger.Infof("幂等性拦截，唯一约束冲突，回滚本次执行: %s", idemKey)
					return errIdempotentConflict
				}
				return fmt.Errorf("插入执行日志失败: %w", err)
			}

			return nil
		})

		if txErr != nil {
			if errors.Is(txErr, errIdempotentConflict) {
				// 幂等冲突：认为本次是重复执行，不算错误，也不计入 successCount。
				continue
			}
			logger.Errorf("规则执行事务失败, rule=%d, user=%s, err=%v", rule.Id, uid, txErr)
			continue
		}

		successCount++
	}

	return successCount, nil
}

// resolveRechargeAction 负责把“动作配置”转换成“充值金额 + 充值类型 + 规范化动作类型”。
func resolveRechargeAction(actionType string, amount float64) (float64, int64, string, error) {
	if amount <= 0 {
		return 0, 0, "", fmt.Errorf("动作金额必须大于0")
	}
	// 目前业务只保留“充值/加积分”一种动作类型，
	// 因此忽略传入的 actionType，统一视为加积分。
	return amount, 3, "add_points", nil
}

// buildExpireTime 根据配置的过期策略，算出充值记录的 expire_time。
func buildExpireTime(expireStrategy string, now time.Time, rechargeAmount float64) sql.NullTime {
	// 扣减记录不需要过期时间。
	if rechargeAmount <= 0 {
		return sql.NullTime{Valid: false}
	}
	strategy := strings.ToLower(strings.TrimSpace(expireStrategy))
	switch strategy {
	case "month_end":
		monthEnd := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()).Add(-time.Second)
		return sql.NullTime{Time: monthEnd, Valid: true}
	case "fixed_30_days", "fixed_days":
		return sql.NullTime{Time: now.AddDate(0, 0, 30), Valid: true}
	default:
		// 支持固定日期：date:YYYY-MM-DD
		if strings.HasPrefix(strategy, "date:") {
			raw := strings.TrimPrefix(strategy, "date:")
			if d, err := time.ParseInLocation("2006-01-02", raw, now.Location()); err == nil {
				dayEnd := time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, d.Location()).Add(-time.Second)
				return sql.NullTime{Time: dayEnd, Valid: true}
			}
		}
		// 支持通用的 fixed_{N}_days，自定义 N 天后失效。
		if strings.HasPrefix(strategy, "fixed_") && strings.HasSuffix(strategy, "_days") {
			raw := strings.TrimPrefix(strategy, "fixed_")
			raw = strings.TrimSuffix(raw, "_days")
			if days, err := strconv.Atoi(raw); err == nil && days > 0 {
				return sql.NullTime{Time: now.AddDate(0, 0, days), Valid: true}
			}
		}
		return sql.NullTime{Valid: false}
	}
}
