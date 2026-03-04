// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

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
	"AgentEarth-Mgr/admin/internal/types"
	ruleModel "AgentEarth-Mgr/models/rules"

	"github.com/zeromicro/go-zero/core/logx"
)

type SaveRuleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSaveRuleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveRuleLogic {
	return &SaveRuleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 负责处理“保存规则”的请求
// 既处理“新建规则”，也处理“编辑已有规则”
func (l *SaveRuleLogic) SaveRule(req *types.SaveRuleReq) error {
	status := "inactive"
	if req.IsActive {
		status = "active"
	}

	triggerKey := strings.TrimSpace(req.CronExpression) //去掉空格，避免“ ”导致有值
	if len(triggerKey) == 0 {                           //为空，则根据频率/时间生成
		triggerKey = buildCronExpression(req)
	}

	filterConfig := strings.TrimSpace(req.FilterConfig)
	if len(filterConfig) == 0 {
		filterConfig = buildFilterConfig(req)
	}

	actionConfig := strings.TrimSpace(req.ActionConfig)
	if len(actionConfig) == 0 {
		actionConfig = buildActionConfig(req)
	}

	desc := sql.NullString{Valid: false}
	if req.Description != "" {
		desc = sql.NullString{
			String: req.Description,
			Valid:  true, //标记这个值有效（不是null）
		}
	}

	// 准备时间字段，当前时间 now
	now := time.Now()

	// 新建
	if req.Id == 0 {
		data := &ruleModel.AeRule{
			Name:         req.Name,
			Description:  desc,
			Active:       status,
			CronValue:    triggerKey,
			FilterConfig: filterConfig,
			ActionConfig: actionConfig,
			CreateTime:   now,
			UpdateTime:   now,
		}
		// 插入规则并拿到自增主键 id，便于立刻注册到调度器
		id, err := l.svcCtx.RuleModel.InsertAndReturnId(l.ctx, data)
		if err != nil {
			l.Logger.Errorf("插入规则失败, data=%+v, err=%v", data, err)
			return err
		}

		// 如果规则是启用状态，则把 cron 表达式注册到调度器里
		if req.IsActive {
			registerRuleInScheduler(l.ctx, id, triggerKey)
		}
		return nil
	}

	// 更新
	// 先查出当前这条规则的记录
	exist, err := l.svcCtx.RuleModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return err
	}

	//覆盖需要更新的字段（按照业务要求，只改这些字段）
	exist.Name = req.Name
	exist.Description = desc
	exist.Active = status
	exist.CronValue = triggerKey
	exist.FilterConfig = filterConfig
	exist.ActionConfig = actionConfig
	exist.UpdateTime = now //更新时间改为当前时间

	if err := l.svcCtx.RuleModel.Update(l.ctx, exist); err != nil {
		l.Logger.Errorf("更新规则失败, id=%d, err=%v", req.Id, err)
		return err
	}

	// 更新后的规则如果是启用状态，则注册/更新调度器中的 entry；否则从调度器里移除。
	if exist.Active == "active" {
		registerRuleInScheduler(l.ctx, exist.Id, exist.CronValue)
	} else {
		unregisterRuleFromScheduler(l.ctx, exist.Id)
	}

	return nil //一切成功就返回nil
}

func buildCronExpression(req *types.SaveRuleReq) string {
	frequency := strings.ToLower(strings.TrimSpace(req.Frequency))
	if len(frequency) == 0 {
		return "0 0 1 * *"
	}

	hour, minute := 0, 0
	t := strings.TrimSpace(req.ExecTime)
	if len(t) > 0 {
		parts := strings.Split(t, ":")
		if len(parts) == 2 {
			if h, err := strconv.Atoi(parts[0]); err == nil && h >= 0 && h <= 23 {
				hour = h
			}
			if m, err := strconv.Atoi(parts[1]); err == nil && m >= 0 && m <= 59 {
				minute = m
			}
		}
	}

	switch frequency {
	case "daily":
		return fmt.Sprintf("%d %d * * *", minute, hour)
	case "weekly":
		week := req.DayOfWeek
		if week < 0 || week > 6 {
			week = 1
		}
		return fmt.Sprintf("%d %d * * %d", minute, hour, week)
	default: // monthly：按“每月的某一天”来跑
		day := req.DayOfMonth
		// 允许用户选择 1~31 号：
		// - 小于等于 0 的非法值，统一回退到 1 号
		// - 大于 31 的非法值，统一收敛到 31 号
		// 这样 30、31 号这种在大多数月份存在的日期也能正常使用；
		// 对于 2 月这种没有 30/31 号的月份，底层 cron 库会“那个月就不触发”，是标准 cron 语义。
		if day <= 0 {
			day = 1
		} else if day > 31 {
			day = 31
		}
		return fmt.Sprintf("%d %d %d * *", minute, hour, day)
	}
}

func buildFilterConfig(req *types.SaveRuleReq) string {
	minBalance := req.MinBalance
	maxBalance := req.MaxBalance
	if minBalance == 0 && maxBalance == 0 {
		// 语义：如果都是 0，表示“不限制余额”，所以设成 -1/-1，
		// 后续 SQL 里会把 <0 当成“不加这个条件”。
		minBalance = -1
		maxBalance = -1
	}

	payload := map[string]interface{}{
		"min_reg_days":            req.MinRegDays,                     // 注册天数 >= 多少天
		"max_reg_days":            req.MaxRegDays,                     // 注册天数 <= 多少天
		"last_login_within_days":  req.LastLoginWithinDays,            // 最近多少天登录过
		"min_last_month_consume":  req.MinLastMonthConsume,            // 上月消费下限
		"max_last_month_consume":  req.MaxLastMonthConsume,            // 上月消费上限
		"min_balance":             minBalance,                         // 当前余额下限
		"max_balance":             maxBalance,                         // 当前余额上限
		"reg_channel":             strings.TrimSpace(req.RegChannel),  // 注册渠道
		"min_history_recharge":    req.MinHistoryRecharge,             // 历史累计充值下限
		"max_history_recharge":    req.MaxHistoryRecharge,             // 历史累计充值上限
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return `{"min_reg_days":0}`
	}
	return string(b)
}

// 构造“动作”配置的 JSON，遵循 ae_cron_rule.action_config 的新结构：
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

type ruleActionBody struct {
	ChargeType     int64   `json:"charge_type"`
	Amount         float64 `json:"amount"`
	ExpireStrategy string  `json:"expire_strategy"`
}

type ruleAction struct {
	ActionType string        `json:"action_type"`
	ActionBody ruleActionBody `json:"action_body"`
}

type ruleActionsPayload struct {
	Actions []ruleAction `json:"actions"`
}

func buildActionConfig(req *types.SaveRuleReq) string {
	expireStrategy := strings.TrimSpace(req.ExpireStrategy)
	if len(expireStrategy) == 0 {
		expireStrategy = "month_end"
	}

	// 当前版本只真正支持充值（recharge）这一种动作，
	// 但这里按照 action_type 分支，方便后续扩展其它动作类型。
	actionType := strings.TrimSpace(req.ActionType)
	if actionType == "" {
		actionType = "recharge"
	}

	// 使用类型安全的结构体来构造 action_body，再通过 json.Marshal 生成 JSON，
	// 语义上是：先根据 action_type 决定使用哪种 body 结构，再填充具体字段。
	var actions []ruleAction
	switch actionType {
	case "recharge":
		body := ruleActionBody{
			ChargeType:     int64(301),
			Amount:         req.ActionAmount,
			ExpireStrategy: expireStrategy,
		}
		actions = append(actions, ruleAction{
			ActionType: "recharge",
			ActionBody: body,
		})
	default:
		// 暂不支持的 action_type：为避免写入无法执行的配置，这里仍然回退为单一的 recharge 动作。
		body := ruleActionBody{
			ChargeType:     int64(301),
			Amount:         req.ActionAmount,
			ExpireStrategy: expireStrategy,
		}
		actions = append(actions, ruleAction{
			ActionType: "recharge",
			ActionBody: body,
		})
	}

	payload := ruleActionsPayload{
		Actions: actions,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		// 兜底一个合法的默认配置，避免规则存储为空。
		return `{"actions":[{"action_type":"recharge","action_body":{"charge_type":301,"amount":0,"expire_strategy":"month_end"}}]}`
	}
	return string(b)
}
