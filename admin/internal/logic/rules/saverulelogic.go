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

	cronValue := strings.TrimSpace(req.CronExpression) //去掉空格，避免“ ”导致有值
	if len(cronValue) == 0 {                           //为空，则根据频率/时间生成
		cronValue = buildCronExpression(req)
	}

	filterConfig := strings.TrimSpace(req.FilterConfig)
	if len(filterConfig) == 0 {
		var err error
		filterConfig, err = buildFilterConfig(req)
		if err != nil {
			l.Logger.Errorf("生成筛选配置失败, req=%+v, err=%v", req, err)
			return err
		}
	}

	// 检查规则命中用户数，如果为 0 则返回错误提示
	if err := l.checkAudienceCount(filterConfig); err != nil {
		return err
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
			CronValue:    cronValue,
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
			registerRuleInScheduler(l.ctx, id, cronValue)
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
	exist.CronValue = cronValue
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

// parseExecTime 解析 "HH:mm" 为 hour, minute；非法则返回 0,0。
func parseExecTime(execTime string) (hour, minute int) {
	parts := strings.SplitN(strings.TrimSpace(execTime), ":", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	h, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	if h < 0 || h > 23 {
		h = 0
	}
	if m < 0 || m > 59 {
		m = 0
	}
	return h, m
}

// buildCronExpression 根据表单（频率 + 执行时间 + 日/周）生成标准 5 段 cron 表达式。无现成库可替代，此处为常见写法。
func buildCronExpression(req *types.SaveRuleReq) string {
	frequency := strings.ToLower(strings.TrimSpace(req.Frequency))
	if frequency == "" {
		return "0 0 1 * *"
	}
	hour, minute := parseExecTime(req.ExecTime)
	switch frequency {
	case "daily":
		return fmt.Sprintf("%d %d * * *", minute, hour)
	case "weekly":
		day := req.DayOfWeek
		if day < 0 || day > 6 {
			day = 1
		}
		return fmt.Sprintf("%d %d * * %d", minute, hour, day)
	default: // monthly
		day := req.DayOfMonth
		if day < 1 {
			day = 1
		} else if day > 31 {
			day = 31
		}
		return fmt.Sprintf("%d %d %d * *", minute, hour, day)
	}
}

func buildFilterConfig(req *types.SaveRuleReq) (string, error) {
	// 余额：0/0 表示“余额=0”；<0 表示不限制（与 types 约定一致，原样写入）。
	payload := map[string]interface{}{
		"min_reg_days":            req.MinRegDays,                     // 注册天数 >= 多少天
		"max_reg_days":            req.MaxRegDays,                     // 注册天数 <= 多少天
		"last_login_within_days":  req.LastLoginWithinDays,            // 最近多少天登录过
		"min_last_month_consume":  req.MinLastMonthConsume,            // 上月消费下限
		"max_last_month_consume":  req.MaxLastMonthConsume,            // 上月消费上限
		"min_balance":             req.MinBalance,                     // 当前余额下限
		"max_balance":             req.MaxBalance,                     // 当前余额上限
		"reg_channel":             strings.TrimSpace(req.RegChannel),  // 注册渠道
		"min_history_recharge":    req.MinHistoryRecharge,            // 历史累计充值下限
		"max_history_recharge":    req.MaxHistoryRecharge,            // 历史累计充值上限
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("生成筛选配置 JSON 失败: %w", err)
	}
	return string(b), nil
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

// buildActionConfig 根据保存请求生成 action_config JSON，当前仅支持单条充值动作。
func buildActionConfig(req *types.SaveRuleReq) string {
	expireStrategy := strings.TrimSpace(req.ExpireStrategy)
	if expireStrategy == "" {
		expireStrategy = "month_end"
	}
	payload := ruleActionsPayload{
		Actions: []ruleAction{{
			ActionType: "recharge",
			ActionBody: ruleActionBody{
				ChargeType:     301,
				Amount:         req.ActionAmount,
				ExpireStrategy: expireStrategy,
			},
		}},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf(`{"actions":[{"action_type":"recharge","action_body":{"charge_type":301,"amount":%f,"expire_strategy":%q}}]}`, req.ActionAmount, expireStrategy)
	}
	return string(b)
}

// checkAudienceCount 检查规则筛选条件：
// 1）如果根本没有配置任何受众筛选条件，直接拒绝保存；
// 2）如果命中用户数为 0，则拒绝保存，提示运营调整筛选条件。
func (l *SaveRuleLogic) checkAudienceCount(filterConfig string) error {
	var filter ruleModel.AudienceFilter
	if err := json.Unmarshal([]byte(filterConfig), &filter); err != nil {
		l.Logger.Errorf("解析筛选配置失败: %v", err)
		return fmt.Errorf("解析筛选配置失败: %w", err)
	}

	// 判定是否真正配置了“受众筛选条件”。以下任一成立，即认为配置了筛选：
	// - 注册时间有上下限（>0）
	// - 最近登录天数 > 0
	// - 上月消费 / 历史充值 / 余额 任一边 >=0（-1 表示不限）
	// - 注册渠道非空
	hasRegLimit := filter.MinRegDays > 0 || filter.MaxRegDays > 0
	hasLastLogin := filter.LastLoginWithinDays > 0
	hasConsume := filter.MinLastMonthConsume >= 0 || filter.MaxLastMonthConsume >= 0
	hasBalance := filter.MinBalance >= 0 || filter.MaxBalance >= 0
	hasHistory := filter.MinHistoryRecharge >= 0 || filter.MaxHistoryRecharge >= 0
	hasChannel := strings.TrimSpace(filter.RegChannel) != ""

	if !(hasRegLimit || hasLastLogin || hasConsume || hasBalance || hasHistory || hasChannel) {
		l.Logger.Infof("规则未配置任何受众筛选条件，拒绝保存")
		return fmt.Errorf("RULE_NO_AUDIENCE:未配置任何受众筛选条件，请至少设置一个条件后再保存")
	}

	count, err := l.svcCtx.RuleQueryModel.CountAudience(l.ctx, filter)
	if err != nil {
		l.Logger.Errorf("查询命中用户数失败: %v", err)
		return fmt.Errorf("查询命中用户数失败: %w", err)
	}

	if count == 0 {
		l.Logger.Infof("规则命中用户数为 0，拒绝保存")
		return fmt.Errorf("RULE_NO_AUDIENCE:当前筛选条件下无命中用户，请调整筛选条件后再保存")
	}

	l.Logger.Infof("规则命中用户数: %d", count)
	return nil
}
