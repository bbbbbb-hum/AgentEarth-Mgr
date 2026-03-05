package rules

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	ruleModel "AgentEarth-Mgr/models/rules"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ruleActionsConfig 描述 action_config 新版 JSON 结构：
// {
//   "actions": [
//     {
//       "action_type": "recharge",
//       "action_body": {
//         "charge_type": 301,
//         "amount": 100,
//         "expire_strategy": "month_end"
//       }
//     }
//   ]
// }
type ruleActionsConfig struct {
	Actions []struct {
		ActionType string          `json:"action_type"`
		ActionBody json.RawMessage `json:"action_body"`
	} `json:"actions"`
}

// rechargeActionBody 为当前仅支持的 recharge 动作体。
type rechargeActionBody struct {
	ChargeType     int64   `json:"charge_type"`
	Amount         float64 `json:"amount"`
	ExpireStrategy string  `json:"expire_strategy"`
}

// 永不过期的哨兵年份，用于区分「永不过期」与「无意义的过期时间」。
const neverExpireYear = 9999

// ExecuteRule 执行单条规则，供手动和自动调度共用。
func ExecuteRule(ctx context.Context, svcCtx *svc.ServiceContext, rule *ruleModel.AeRule, execSource, operator string) (int, error) {
	logger := logx.WithContext(ctx)

	// 1. 解析规则上的 filter_config JSON 到受众筛选条件（与 model 层 AudienceFilter 共用）。
	var filter ruleModel.AudienceFilter
	if err := json.Unmarshal([]byte(rule.FilterConfig), &filter); err != nil {
		return 0, fmt.Errorf("解析筛选配置失败: %w", err)
	}

	rechargeAmount, chargeType, expireStrategy, err := parseRechargeActionConfig(rule.ActionConfig)
	if err != nil {
		return 0, err
	}

	if filter.MinLastMonthConsume >= 0 && filter.MaxLastMonthConsume >= 0 && filter.MaxLastMonthConsume < filter.MinLastMonthConsume {
		return 0, fmt.Errorf("筛选条件无效: max_last_month_consume 小于 min_last_month_consume")
	}
	if filter.MaxBalance >= 0 && filter.MinBalance >= 0 && filter.MaxBalance < filter.MinBalance {
		return 0, fmt.Errorf("筛选条件无效: max_balance 小于 min_balance")
	}
	if filter.MinHistoryRecharge >= 0 && filter.MaxHistoryRecharge >= 0 && filter.MaxHistoryRecharge < filter.MinHistoryRecharge {
		return 0, fmt.Errorf("筛选条件无效: max_history_recharge 小于 min_history_recharge")
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

	// 9. 处理执行过程中的时间/备注等。
	now := time.Now()
	// 根据过期策略算出本次加积分的过期时间
	expireTime := buildExpireTime(expireStrategy, now, rechargeAmount)

	// 用于写入 ae_user_recharge_record.remark，标识本条充值记录的触发来源。
	sourceDesc := "定时任务触发"
	if execSource == "manual" {
		sourceDesc = "管理员手动触发"
	}

	// 10. 针对每个用户依次执行“加点数”，在充值记录表中打上 rule_id/exec_source 标记。
	successCount := 0
	var chargeSource int64
	if strings.TrimSpace(execSource) == "manual" {
		// 运营手工批量触发
		chargeSource = 3
	} else {
		// 自动 Rule 调度触发
		chargeSource = 4
	}

	for _, uid := range userIds {
		// 插入到充值记录表里的备注
		rechargeRemark := fmt.Sprintf("规则执行充值 | 规则ID:%d | 触发:%s", rule.Id, sourceDesc)

		// 对于每个用户，以事务的方式执行“加钱”（写入 ae_user_recharge_record）：
		//  - 用 svcCtx.DB.TransactCtx 开启事务，内部通过 WithSession 绑定同一个 session。
		//  - 任一操作失败，都会导致本次“加钱”回滚。
		txErr := svcCtx.DB.TransactCtx(ctx, func(txCtx context.Context, session sqlx.Session) error {
			// 事务内统一使用基于 session 的 model
			txRechargeModel := svcCtx.UserRechargeRecordModel.WithSession(session)

			// 10.1 插入充值记录（带过期时间），并在记录上打上 rule_id / exec_source 标记，便于规则维度对账。
			_, rechargeErr := txRechargeModel.InsertRuleRechargeRecordWithExpire(
				txCtx,
				uid,
				rechargeAmount,
				now,
				now,
				now,
				rule.Id,
				chargeSource,
				chargeType,
				rechargeRemark,
				operator,
				expireTime,
			)
			if rechargeErr != nil {
				return fmt.Errorf("插入充值记录失败: %w", rechargeErr)
			}

			return nil
		})

		if txErr != nil {
			// 加钱失败：认为本次规则对该用户的执行失败（资金未变动），仅记录错误日志。
			logger.Errorf("规则执行事务失败, rule=%d, user=%s, err=%v", rule.Id, uid, txErr)
			continue
		}

		successCount++
	}

	return successCount, nil
}

// parseRechargeActionConfig 解析规则的 action_config。当前仅支持 recharge 一种动作，直接取第一个 action 的 action_body。
// 示例：{"actions":[{"action_type":"recharge","action_body":{"charge_type":301,"amount":100,"expire_strategy":"month_end"}}]}
func parseRechargeActionConfig(raw string) (amount float64, chargeType int64, expireStrategy string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, 0, "", fmt.Errorf("动作配置为空")
	}

	var cfg ruleActionsConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return 0, 0, "", fmt.Errorf("解析动作配置失败: %w", err)
	}
	if len(cfg.Actions) == 0 {
		return 0, 0, "", fmt.Errorf("动作列表不能为空")
	}

	var body rechargeActionBody
	if err := json.Unmarshal(cfg.Actions[0].ActionBody, &body); err != nil {
		return 0, 0, "", fmt.Errorf("解析 recharge 动作配置失败: %w", err)
	}
	if body.Amount <= 0 {
		return 0, 0, "", fmt.Errorf("动作金额必须大于0")
	}
	if body.ChargeType == 0 {
		// 兼容旧语义：不填时默认活动赠送（301）
		body.ChargeType = 301
	}
	return body.Amount, body.ChargeType, body.ExpireStrategy, nil
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
	case "never":
		never := time.Date(neverExpireYear, 12, 31, 23, 59, 59, 0, now.Location())
		return sql.NullTime{Time: never, Valid: true}
	}

	// 固定日期：date:YYYY-MM-DD（该日 23:59:59 失效）
	if strings.HasPrefix(strategy, "date:") {
		raw := strings.TrimPrefix(strategy, "date:")
		if d, err := time.ParseInLocation("2006-01-02", raw, now.Location()); err == nil {
			dayEnd := time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, d.Location()).Add(-time.Second)
			return sql.NullTime{Time: dayEnd, Valid: true}
		}
	}
	// fixed_{N}_days：包含当日的 N 个自然日，到第 N 日 23:59:59 失效（如 fixed_30_days、fixed_7_days）
	if strings.HasPrefix(strategy, "fixed_") && strings.HasSuffix(strategy, "_days") {
		raw := strings.TrimPrefix(strategy, "fixed_")
		raw = strings.TrimSuffix(raw, "_days")
		if days, err := strconv.Atoi(raw); err == nil && days > 0 {
			dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			dayEnd := dayStart.AddDate(0, 0, days).Add(-time.Second)
			return sql.NullTime{Time: dayEnd, Valid: true}
		}
	}
	return sql.NullTime{Valid: false}
}
