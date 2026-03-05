package rules

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeRuleModel = (*customAeRuleModel)(nil)

type (
	// RuleQueryModel 承载规则模块的查询类 SQL（列表/统计/对账/调度/受众预览等）。
	RuleQueryModel interface {
		// 规则列表
		GetRuleList(ctx context.Context, search string, size, offset int64) (total int64, rows []RuleListRow, err error)

		// 仪表盘
		GetRuleDashboard(ctx context.Context) (runningRules int64, monthlyTouchedUsers int64, monthlyAutoPoints float64, err error)

		// 对账：执行批次列表
		GetExecutionRuns(ctx context.Context, ruleId int64, size, offset int64) (total int64, rows []ExecutionRunRow, err error)

		// 对账：执行明细
		GetExecutionDetails(ctx context.Context, ruleId int64, runTime string, chargeSource int64) (total int64, rows []ExecutionDetailRow, err error)

		// PG advisory lock
		TryAdvisoryLock(ctx context.Context, key int64) (bool, error)
		AdvisoryUnlock(ctx context.Context, key int64) error

		// 删除：按规则ID删除规则记录
		DeleteRuleById(ctx context.Context, ruleId int64) (rowsAffected int64, err error)

		// 受众预览：统计 + 分页列表（复用同一套筛选语义）
		CountAudience(ctx context.Context, f AudienceFilter) (int64, error)
		ListAudience(ctx context.Context, f AudienceFilter, size, offset int64) ([]AudienceUserRow, error)

		// 规则执行：只取 user_id 列表（供 ExecuteRule 使用）
		ListAudienceUserIds(ctx context.Context, f AudienceFilter) ([]string, error)

		// 调度器：查询所有已启用规则的基本信息（供 autoscheduler 注册 cron 任务使用）
		ListActiveRulesForScheduler(ctx context.Context) ([]SchedulerRuleRow, error)
	}

	// AeRuleModel 是 ae_cron_rule 表对应的主模型接口，扩展了查询能力。
	AeRuleModel interface {
		aeRuleModel
		RuleQueryModel
		withSession(session sqlx.Session) AeRuleModel

		// InsertAndReturnId 插入规则并返回自增主键 id，便于上层立刻把规则注册到调度器。
		InsertAndReturnId(ctx context.Context, data *AeRule) (int64, error)
	}

	customAeRuleModel struct {
		*defaultAeRuleModel
	}

	RuleListRow struct {
		Id           int64     `db:"id"`
		Name         string    `db:"name"`
		Description  string    `db:"description"`
		Active       string    `db:"active"`
		CronValue    string    `db:"cron_value"`
		FilterConfig string    `db:"filter_config"`
		ActionConfig string    `db:"action_config"`
		CreateTime   time.Time `db:"create_time"`
	}

	ExecutionRunRow struct {
		RunTime      string `db:"run_time"`
		ChargeSource int64  `db:"charge_source"`
		ExecSource   string `db:"exec_source"`
		TotalCount   int64  `db:"total_count"`
		SuccessCount int64  `db:"success_count"`
		FailedCount  int64  `db:"failed_count"`
	}

	ExecutionDetailRow struct {
		TriggerTime  string         `db:"trigger_time"`
		ExecSource   string         `db:"exec_source"`
		UserId       string         `db:"user_id"`
		UserName     sql.NullString `db:"user_name"`
		OperatorName sql.NullString `db:"operator_name"`
		ChangeAmount float64        `db:"change_amount"`
		Status       string         `db:"status"`
		ActionType   string         `db:"action_type"`
	}

	// AudienceFilter 规则受众筛选条件，与 filter_config JSON 及 ListAudienceUserIds 等接口共用。
	AudienceFilter struct {
		MinRegDays          int64   `json:"min_reg_days"`
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

	AudienceUserRow struct {
		UserId   string `db:"user_id"`
		Username string `db:"username"`
	}

	// SchedulerRuleRow 用于调度器加载规则时的精简字段集。
	SchedulerRuleRow struct {
		Id        int64  `db:"id"`
		CronValue string `db:"cron_value"`
		Active    string `db:"active"`
	}
)

// NewAeRuleModel returns a model for the database table.
func NewAeRuleModel(conn sqlx.SqlConn) AeRuleModel {
	return &customAeRuleModel{
		defaultAeRuleModel: newAeRuleModel(conn),
	}
}

// 兼容原有构造函数，供 svc 使用。
func NewRuleQueryModel(conn sqlx.SqlConn) RuleQueryModel {
	return NewAeRuleModel(conn)
}

func (m *customAeRuleModel) withSession(session sqlx.Session) AeRuleModel {
	return NewAeRuleModel(sqlx.NewSqlConnFromSession(session))
}

// InsertAndReturnId 自定义插入方法，使用 INSERT ... RETURNING id 取回自增主键。
func (m *customAeRuleModel) InsertAndReturnId(ctx context.Context, data *AeRule) (int64, error) {
	// 与 Insert 一致，显式指定列名，避免字段列表和占位符数量不一致。
	query := fmt.Sprintf(`
		INSERT INTO %s (
			name,
			description,
			active,
			cron_value,
			filter_config,
			action_config,
			create_time,
			update_time
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
	`, m.table)

	var id int64
	if err := m.conn.QueryRowCtx(ctx, &id, query,
		data.Name,
		data.Description,
		data.Active,
		data.CronValue,
		data.FilterConfig,
		data.ActionConfig,
		data.CreateTime,
		data.UpdateTime,
	); err != nil {
		return 0, err
	}

	return id, nil
}

func (m *customAeRuleModel) GetRuleList(ctx context.Context, search string, size, offset int64) (int64, []RuleListRow, error) {
	var total int64
	countSql := `
		SELECT COUNT(1)
		FROM "public"."ae_cron_rule" r
		WHERE ($1 = '' OR r.name ILIKE '%' || $1 || '%')
	`
	if err := m.conn.QueryRowCtx(ctx, &total, countSql, search); err != nil {
		return 0, nil, fmt.Errorf("查询规则总数失败: %w", err)
	}

	querySql := `
		SELECT
			r.id,
			r.name,
			COALESCE(r.description, '') AS description,
			r.active,
			r.cron_value,
			r.filter_config,
			r.action_config,
			r.create_time
		FROM "public"."ae_cron_rule" r
		WHERE ($1 = '' OR r.name ILIKE '%' || $1 || '%')
		ORDER BY r.id DESC
		LIMIT $2 OFFSET $3
	`
	var rows []RuleListRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, querySql, search, size, offset); err != nil {
		return 0, nil, fmt.Errorf("查询规则列表失败: %w", err)
	}
	return total, rows, nil
}

func (m *customAeRuleModel) GetRuleDashboard(ctx context.Context) (int64, int64, float64, error) {
	var runningRules int64
	if err := m.conn.QueryRowCtx(ctx, &runningRules, `
		SELECT COUNT(1) FROM "public"."ae_cron_rule" WHERE active = 'active'
	`); err != nil {
		return 0, 0, 0, fmt.Errorf("查询运行中规则数失败: %w", err)
	}

	var monthlyTouchedUsers int64
	if err := m.conn.QueryRowCtx(ctx, &monthlyTouchedUsers, `
		SELECT COUNT(DISTINCT r.user_id)
		FROM "public"."ae_user_recharge_record" r
		JOIN "public"."ae_cron_rule" cr ON r.rule_id = cr.id
		WHERE cr.active = 'active'
		  AND r.xlcredit_amount > 0
		  AND r.pay_time >= date_trunc('month', now())
		  AND r.pay_time < date_trunc('month', now()) + interval '1 month'
	`); err != nil {
		return 0, 0, 0, fmt.Errorf("查询本月触达用户失败: %w", err)
	}

	var monthlyAutoPoints float64
	if err := m.conn.QueryRowCtx(ctx, &monthlyAutoPoints, `
		SELECT COALESCE(SUM(r.xlcredit_amount), 0)
		FROM "public"."ae_user_recharge_record" r
		JOIN "public"."ae_cron_rule" cr ON r.rule_id = cr.id
		WHERE cr.active = 'active'
		  AND r.xlcredit_amount > 0
		  AND r.charge_source = 4
		  AND r.pay_time >= date_trunc('month', now())
		  AND r.pay_time < date_trunc('month', now()) + interval '1 month'
	`); err != nil {
		return 0, 0, 0, fmt.Errorf("查询本月自动赠送点数失败: %w", err)
	}

	return runningRules, monthlyTouchedUsers, monthlyAutoPoints, nil
}

func (m *customAeRuleModel) ListActiveRulesForScheduler(ctx context.Context) ([]SchedulerRuleRow, error) {
	var rows []SchedulerRuleRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT id, cron_value, active
		FROM "public"."ae_cron_rule"
		WHERE active = 'active'
	`); err != nil {
		return nil, fmt.Errorf("查询已启用规则列表失败: %w", err)
	}
	return rows, nil
}

func (m *customAeRuleModel) GetExecutionRuns(ctx context.Context, ruleId int64, size, offset int64) (int64, []ExecutionRunRow, error) {
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, `
		SELECT COUNT(1) FROM (
			SELECT date_trunc('second', pay_time) AS run_time,
			       charge_source
			FROM "public"."ae_user_recharge_record"
			WHERE rule_id = $1
			GROUP BY date_trunc('second', pay_time), charge_source
		) t
	`, ruleId); err != nil {
		return 0, nil, fmt.Errorf("查询执行批次总数失败: %w", err)
	}

	var rows []ExecutionRunRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT
			to_char(date_trunc('second', pay_time), 'YYYY-MM-DD HH24:MI:SS') AS run_time,
			charge_source                                                          AS charge_source,
			CASE WHEN charge_source = 3 THEN 'manual' ELSE 'cron' END             AS exec_source,
			COUNT(1) AS total_count,
			COUNT(1) AS success_count,
			0        AS failed_count
		FROM "public"."ae_user_recharge_record"
		WHERE rule_id = $1
		GROUP BY date_trunc('second', pay_time), charge_source
		ORDER BY date_trunc('second', pay_time) DESC, charge_source DESC
		LIMIT $2 OFFSET $3
	`, ruleId, size, offset); err != nil {
		return 0, nil, fmt.Errorf("查询执行批次列表失败: %w", err)
	}

	return total, rows, nil
}

func (m *customAeRuleModel) GetExecutionDetails(ctx context.Context, ruleId int64, runTime string, chargeSource int64) (int64, []ExecutionDetailRow, error) {
	if chargeSource == 0 {
		// 兜底：未显式传入时，默认按“自动规则”维度过滤，避免误查所有来源。
		chargeSource = 4
	}

	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, `
		SELECT COUNT(1)
		FROM "public"."ae_user_recharge_record" r
		WHERE r.rule_id = $1
		  AND to_char(date_trunc('second', r.pay_time), 'YYYY-MM-DD HH24:MI:SS') = $2
		  AND r.charge_source = $3
	`, ruleId, runTime, chargeSource); err != nil {
		return 0, nil, fmt.Errorf("查询执行明细总数失败: %w", err)
	}

	var rows []ExecutionDetailRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT
			to_char(r.pay_time, 'YYYY-MM-DD HH24:MI:SS') AS trigger_time,
			CASE WHEN r.charge_source = 3 THEN 'manual' ELSE 'cron' END AS exec_source,
			r.user_id,
			COALESCE(u.username, r.user_id::text) AS user_name,
			COALESCE(op.username, r.operator) AS operator_name,
			r.xlcredit_amount AS change_amount,
			'success'           AS status,
			'add_points'        AS action_type
		FROM "public"."ae_user_recharge_record" r
		LEFT JOIN "public"."ae_user" u ON u.user_id::text = r.user_id::text
		LEFT JOIN "public"."ae_mgrsystem_user" op ON (op.user_id::text = r.operator OR op.username = r.operator)
		WHERE r.rule_id = $1
		  AND to_char(date_trunc('second', r.pay_time), 'YYYY-MM-DD HH24:MI:SS') = $2
		  AND r.charge_source = $3
		ORDER BY r.pay_time DESC, r.id DESC
	`, ruleId, runTime, chargeSource); err != nil {
		return 0, nil, fmt.Errorf("查询执行明细列表失败: %w", err)
	}

	return total, rows, nil
}

func (m *customAeRuleModel) TryAdvisoryLock(ctx context.Context, key int64) (bool, error) {
	var ok bool
	err := m.conn.QueryRowCtx(ctx, &ok, `SELECT pg_try_advisory_lock($1)`, key)
	return ok, err
}

func (m *customAeRuleModel) AdvisoryUnlock(ctx context.Context, key int64) error {
	var released sql.NullBool
	return m.conn.QueryRowCtx(ctx, &released, `SELECT pg_advisory_unlock($1)`, key)
}

func (m *customAeRuleModel) DeleteRuleById(ctx context.Context, ruleId int64) (int64, error) {
	res, err := m.conn.ExecCtx(ctx, `DELETE FROM "public"."ae_cron_rule" WHERE id = $1`, ruleId)
	if err != nil {
		return 0, err
	}
	ra, _ := res.RowsAffected()
	return ra, nil
}

func (m *customAeRuleModel) buildAudienceFromWhere(f AudienceFilter) (string, []interface{}) {
	minRegDays := f.MinRegDays
	if minRegDays < 0 {
		minRegDays = 0
	}
	minBalance := f.MinBalance // 前端约定：-1 表示不限制下限
	maxBalance := f.MaxBalance // 前端约定：-1 表示不限制上限

	needConsume := f.MinLastMonthConsume > 0 || f.MaxLastMonthConsume > 0
	needBalance := minBalance >= 0 || maxBalance >= 0
	needHistory := f.MinHistoryRecharge > 0 || f.MaxHistoryRecharge > 0

	var sb strings.Builder
	args := []interface{}{minRegDays}
	argIdx := 2

	sb.WriteString(`
		FROM "public"."ae_user" u
	`)
	if needConsume {
		sb.WriteString(`
		LEFT JOIN (
			SELECT user_id, COALESCE(SUM(xlcredit_consume), 0) AS total
			FROM "public"."ae_user_consumption_record_daily"
			WHERE day >= date_trunc('month', now() - interval '1 month')
			  AND day < date_trunc('month', now())
			GROUP BY user_id
		) c ON c.user_id::text = u.user_id::text
		`)
	}
	if needBalance {
		sb.WriteString(`
		LEFT JOIN LATERAL (
			SELECT balance FROM "public"."ae_user_balance_statistic_daily" b2
			WHERE b2.user_id::text = u.user_id::text
			ORDER BY b2.day DESC
			LIMIT 1
		) b ON true
		`)
	}
	if needHistory {
		sb.WriteString(`
		LEFT JOIN (
			SELECT user_id, COALESCE(SUM(CASE WHEN xlcredit_amount > 0 THEN xlcredit_amount ELSE 0 END), 0) AS total_recharge
			FROM "public"."ae_user_recharge_record"
			GROUP BY user_id
		) hr ON hr.user_id::text = u.user_id::text
		`)
	}

	sb.WriteString(`
		WHERE ($1 <= 0 OR u.create_time <= now() - ($1 || ' days')::interval)
	`)
	if f.MaxRegDays > 0 {
		sb.WriteString(fmt.Sprintf(`  AND (u.create_time >= now() - ($%d || ' days')::interval)`+"\n", argIdx))
		args = append(args, f.MaxRegDays)
		argIdx++
	}
	if f.LastLoginWithinDays > 0 {
		sb.WriteString(fmt.Sprintf(`  AND (u.last_login_at >= now() - ($%d || ' days')::interval)`+"\n", argIdx))
		args = append(args, f.LastLoginWithinDays)
		argIdx++
	}
	if strings.TrimSpace(f.RegChannel) != "" {
		sb.WriteString(fmt.Sprintf(`  AND (u.oauth_provider = $%d)`+"\n", argIdx))
		args = append(args, strings.TrimSpace(f.RegChannel))
		argIdx++
	}
	if needConsume {
		if f.MinLastMonthConsume > 0 {
			sb.WriteString(fmt.Sprintf(`  AND (COALESCE(c.total, 0) >= $%d)`+"\n", argIdx))
			args = append(args, f.MinLastMonthConsume)
			argIdx++
		}
		if f.MaxLastMonthConsume > 0 {
			sb.WriteString(fmt.Sprintf(`  AND (COALESCE(c.total, 0) <= $%d)`+"\n", argIdx))
			args = append(args, f.MaxLastMonthConsume)
			argIdx++
		}
	}
	if needBalance {
		if minBalance >= 0 {
			sb.WriteString(fmt.Sprintf(`  AND (COALESCE(b.balance, 0) >= $%d)`+"\n", argIdx))
			args = append(args, minBalance)
			argIdx++
		}
		if maxBalance >= 0 {
			sb.WriteString(fmt.Sprintf(`  AND (COALESCE(b.balance, 0) <= $%d)`+"\n", argIdx))
			args = append(args, maxBalance)
			argIdx++
		}
	}
	if needHistory {
		if f.MinHistoryRecharge > 0 {
			sb.WriteString(fmt.Sprintf(`  AND (COALESCE(hr.total_recharge, 0) >= $%d)`+"\n", argIdx))
			args = append(args, f.MinHistoryRecharge)
			argIdx++
		}
		if f.MaxHistoryRecharge > 0 {
			sb.WriteString(fmt.Sprintf(`  AND (COALESCE(hr.total_recharge, 0) <= $%d)`+"\n", argIdx))
			args = append(args, f.MaxHistoryRecharge)
			argIdx++
		}
	}

	return sb.String(), args
}

func (m *customAeRuleModel) CountAudience(ctx context.Context, f AudienceFilter) (int64, error) {
	fromWhere, args := m.buildAudienceFromWhere(f)
	sqlStr := `SELECT COUNT(*) ` + fromWhere
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, sqlStr, args...); err != nil {
		return 0, err
	}
	return total, nil
}

func (m *customAeRuleModel) ListAudience(ctx context.Context, f AudienceFilter, size, offset int64) ([]AudienceUserRow, error) {
	fromWhere, args := m.buildAudienceFromWhere(f)
	sqlStr := `SELECT u.user_id::text AS user_id, COALESCE(u.username, u.user_id::text) AS username ` + fromWhere +
		fmt.Sprintf(` ORDER BY u.user_id ASC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	listArgs := append(args, size, offset)
	var rows []AudienceUserRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, sqlStr, listArgs...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *customAeRuleModel) ListAudienceUserIds(ctx context.Context, f AudienceFilter) ([]string, error) {
	fromWhere, args := m.buildAudienceFromWhere(f)
	sqlStr := `SELECT u.user_id::text AS user_id ` + fromWhere
	var userIds []string
	if err := m.conn.QueryRowsCtx(ctx, &userIds, sqlStr, args...); err != nil {
		return nil, err
	}
	return userIds, nil
}
