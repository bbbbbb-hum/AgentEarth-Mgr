package rules

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// RuleQueryModel 承载规则模块的查询类 SQL（列表/统计/对账/调度/受众预览等）。
// 目的：让 logic 层不直接写原生 SQL 字符串，只负责组装参数和调用 model。
type RuleQueryModel interface {
	// 规则列表
	GetRuleList(ctx context.Context, search string, size, offset int64) (total int64, rows []RuleListRow, err error)

	// 仪表盘
	GetRuleDashboard(ctx context.Context) (runningRules int64, monthlyTouchedUsers int64, monthlyAutoPoints float64, err error)

	// 对账：执行批次列表
	GetExecutionRuns(ctx context.Context, ruleId int64, size, offset int64) (total int64, rows []ExecutionRunRow, err error)

	// 对账：执行明细
	GetExecutionDetails(ctx context.Context, ruleId int64, runTime string, execSource string) (total int64, rows []ExecutionDetailRow, err error)

	// 调度：查找到期规则 id
	ListDueRuleIds(ctx context.Context, limit int64) ([]int64, error)

	// 调度：更新规则运行时间
	UpdateRuleRunTimes(ctx context.Context, ruleId int64, lastRun time.Time, nextRun time.Time, updated time.Time) error

	// PG advisory lock
	TryAdvisoryLock(ctx context.Context, key int64) (bool, error)
	AdvisoryUnlock(ctx context.Context, key int64) error

	// 删除：先删执行日志再删规则（避免 FK/清理数据）
	DeleteExecutionLogsByRuleId(ctx context.Context, ruleId int64) (rowsAffected int64, err error)
	DeleteRuleById(ctx context.Context, ruleId int64) (rowsAffected int64, err error)

	// 受众预览：统计 + 分页列表（复用同一套筛选语义）
	CountAudience(ctx context.Context, f AudienceFilter) (int64, error)
	ListAudience(ctx context.Context, f AudienceFilter, size, offset int64) ([]AudienceUserRow, error)

	// 规则执行：只取 user_id 列表（供 ExecuteRule 使用）
	ListAudienceUserIds(ctx context.Context, f AudienceFilter) ([]string, error)
}

type (
	RuleListRow struct {
		Id           int64     `db:"id"`
		Name         string    `db:"name"`
		Description  string    `db:"description"`
		Status       string    `db:"status"`
		Priority     int64     `db:"priority"`
		TriggerKey   string    `db:"trigger_key"`
		FilterConfig string    `db:"filter_config"`
		ActionConfig string    `db:"action_config"`
		CreateTime   time.Time `db:"create_time"`
		LastExecTime time.Time `db:"last_exec_time"`
	}

	ExecutionRunRow struct {
		RunTime      string `db:"run_time"`
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
		Remark       sql.NullString `db:"remark"`
		ActionType   string         `db:"action_type"`
	}

	AudienceFilter struct {
		MinRegDays          int
		MaxRegDays          int64
		LastLoginWithinDays int64
		MinLastMonthConsume float64
		MaxLastMonthConsume float64
		MinBalance          float64
		MaxBalance          float64
		RegChannel          string
		MinHistoryRecharge  float64
		MaxHistoryRecharge  float64
	}

	AudienceUserRow struct {
		UserId   string `db:"user_id"`
		Username string `db:"username"`
	}
)

type defaultRuleQueryModel struct {
	conn sqlx.SqlConn
}

func NewRuleQueryModel(conn sqlx.SqlConn) RuleQueryModel {
	return &defaultRuleQueryModel{conn: conn}
}

func (m *defaultRuleQueryModel) GetRuleList(ctx context.Context, search string, size, offset int64) (int64, []RuleListRow, error) {
	var total int64
	countSql := `
		SELECT COUNT(1)
		FROM "public"."ae_rule" r
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
			r.status,
			r.priority,
			r.trigger_key,
			r.filter_config,
			r.action_config,
			r.create_time,
			COALESCE(MAX(l.created_time), '0001-01-01'::timestamptz) AS last_exec_time
		FROM "public"."ae_rule" r
		LEFT JOIN "public"."ae_rule_execution_log" l ON l.rule_id = r.id
		WHERE ($1 = '' OR r.name ILIKE '%' || $1 || '%')
		GROUP BY r.id, r.name, r.description, r.status, r.priority, r.trigger_key, r.filter_config, r.action_config, r.create_time
		ORDER BY r.priority DESC, r.id DESC
		LIMIT $2 OFFSET $3
	`
	var rows []RuleListRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, querySql, search, size, offset); err != nil {
		return 0, nil, fmt.Errorf("查询规则列表失败: %w", err)
	}
	return total, rows, nil
}

func (m *defaultRuleQueryModel) GetRuleDashboard(ctx context.Context) (int64, int64, float64, error) {
	var runningRules int64
	if err := m.conn.QueryRowCtx(ctx, &runningRules, `
		SELECT COUNT(1) FROM "public"."ae_rule" WHERE status = 'active'
	`); err != nil {
		return 0, 0, 0, fmt.Errorf("查询运行中规则数失败: %w", err)
	}

	var monthlyTouchedUsers int64
	if err := m.conn.QueryRowCtx(ctx, &monthlyTouchedUsers, `
		SELECT COUNT(DISTINCT user_id)
		FROM "public"."ae_rule_execution_log"
		WHERE status = 'success'
		  AND created_time >= date_trunc('month', now())
		  AND created_time < date_trunc('month', now()) + interval '1 month'
	`); err != nil {
		return 0, 0, 0, fmt.Errorf("查询本月触达用户失败: %w", err)
	}

	var monthlyAutoPoints float64
	if err := m.conn.QueryRowCtx(ctx, &monthlyAutoPoints, `
		SELECT COALESCE(SUM(change_amount), 0)
		FROM "public"."ae_rule_execution_log"
		WHERE status = 'success'
		  AND action_type = 'add_points'
		  AND exec_source = 'cron'
		  AND created_time >= date_trunc('month', now())
		  AND created_time < date_trunc('month', now()) + interval '1 month'
	`); err != nil {
		return 0, 0, 0, fmt.Errorf("查询本月自动赠送点数失败: %w", err)
	}

	return runningRules, monthlyTouchedUsers, monthlyAutoPoints, nil
}

func (m *defaultRuleQueryModel) GetExecutionRuns(ctx context.Context, ruleId int64, size, offset int64) (int64, []ExecutionRunRow, error) {
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, `
		SELECT COUNT(1) FROM (
			SELECT date_trunc('second', created_time) AS run_time, exec_source
			FROM "public"."ae_rule_execution_log"
			WHERE rule_id = $1
			GROUP BY date_trunc('second', created_time), exec_source
		) t
	`, ruleId); err != nil {
		return 0, nil, fmt.Errorf("查询执行批次总数失败: %w", err)
	}

	var rows []ExecutionRunRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT
			to_char(date_trunc('second', created_time), 'YYYY-MM-DD HH24:MI:SS') AS run_time,
			exec_source,
			COUNT(1) AS total_count,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success_count,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed_count
		FROM "public"."ae_rule_execution_log"
		WHERE rule_id = $1
		GROUP BY date_trunc('second', created_time), exec_source
		ORDER BY date_trunc('second', created_time) DESC
		LIMIT $2 OFFSET $3
	`, ruleId, size, offset); err != nil {
		return 0, nil, fmt.Errorf("查询执行批次列表失败: %w", err)
	}

	return total, rows, nil
}

func (m *defaultRuleQueryModel) GetExecutionDetails(ctx context.Context, ruleId int64, runTime string, execSource string) (int64, []ExecutionDetailRow, error) {
	src := strings.TrimSpace(execSource)
	if src == "" {
		src = "cron"
	}

	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, `
		SELECT COUNT(1)
		FROM "public"."ae_rule_execution_log" l
		WHERE l.rule_id = $1
		  AND to_char(date_trunc('second', l.created_time), 'YYYY-MM-DD HH24:MI:SS') = $2
		  AND l.exec_source = $3
	`, ruleId, runTime, src); err != nil {
		return 0, nil, fmt.Errorf("查询执行明细总数失败: %w", err)
	}

	var rows []ExecutionDetailRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT
			to_char(l.created_time, 'YYYY-MM-DD HH24:MI:SS') AS trigger_time,
			l.exec_source,
			l.user_id,
			COALESCE(u.username, l.user_id::text) AS user_name,
			COALESCE(op.username, l.operator) AS operator_name,
			l.change_amount,
			l.status,
			l.remark,
			l.action_type
		FROM "public"."ae_rule_execution_log" l
		LEFT JOIN "public"."ae_user" u ON u.user_id::text = l.user_id::text
		LEFT JOIN "public"."ae_mgrsystem_user" op ON (op.user_id::text = l.operator OR op.username = l.operator)
		WHERE l.rule_id = $1
		  AND to_char(date_trunc('second', l.created_time), 'YYYY-MM-DD HH24:MI:SS') = $2
		  AND l.exec_source = $3
		ORDER BY l.created_time DESC, l.id DESC
	`, ruleId, runTime, src); err != nil {
		return 0, nil, fmt.Errorf("查询执行明细列表失败: %w", err)
	}

	return total, rows, nil
}

func (m *defaultRuleQueryModel) ListDueRuleIds(ctx context.Context, limit int64) ([]int64, error) {
	if limit <= 0 {
		limit = 200
	}
	type dueRule struct {
		Id int64 `db:"id"`
	}
	var rows []dueRule
	if err := m.conn.QueryRowsCtx(ctx, &rows, `
		SELECT id
		FROM "public"."ae_rule"
		WHERE status = 'active'
		  AND next_run_time IS NOT NULL
		  AND next_run_time <= now()
		ORDER BY priority DESC, id ASC
		LIMIT $1
	`, limit); err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Id)
	}
	return out, nil
}

func (m *defaultRuleQueryModel) UpdateRuleRunTimes(ctx context.Context, ruleId int64, lastRun time.Time, nextRun time.Time, updated time.Time) error {
	_, err := m.conn.ExecCtx(ctx, `
		UPDATE "public"."ae_rule"
		SET last_run_time = $1,
		    next_run_time = $2,
		    update_time = $3
		WHERE id = $4
	`, lastRun, nextRun, updated, ruleId)
	return err
}

func (m *defaultRuleQueryModel) TryAdvisoryLock(ctx context.Context, key int64) (bool, error) {
	var ok bool
	err := m.conn.QueryRowCtx(ctx, &ok, `SELECT pg_try_advisory_lock($1)`, key)
	return ok, err
}

func (m *defaultRuleQueryModel) AdvisoryUnlock(ctx context.Context, key int64) error {
	var released sql.NullBool
	return m.conn.QueryRowCtx(ctx, &released, `SELECT pg_advisory_unlock($1)`, key)
}

func (m *defaultRuleQueryModel) DeleteExecutionLogsByRuleId(ctx context.Context, ruleId int64) (int64, error) {
	res, err := m.conn.ExecCtx(ctx, `DELETE FROM "public"."ae_rule_execution_log" WHERE rule_id = $1`, ruleId)
	if err != nil {
		return 0, err
	}
	ra, _ := res.RowsAffected()
	return ra, nil
}

func (m *defaultRuleQueryModel) DeleteRuleById(ctx context.Context, ruleId int64) (int64, error) {
	res, err := m.conn.ExecCtx(ctx, `DELETE FROM "public"."ae_rule" WHERE id = $1`, ruleId)
	if err != nil {
		return 0, err
	}
	ra, _ := res.RowsAffected()
	return ra, nil
}

func (m *defaultRuleQueryModel) buildAudienceFromWhere(f AudienceFilter) (string, []interface{}) {
	minRegDays := f.MinRegDays
	if minRegDays < 0 {
		minRegDays = 0
	}
	minBalance := f.MinBalance
	maxBalance := f.MaxBalance
	if minBalance == 0 && maxBalance == 0 {
		minBalance = -1
		maxBalance = -1
	}

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

func (m *defaultRuleQueryModel) CountAudience(ctx context.Context, f AudienceFilter) (int64, error) {
	fromWhere, args := m.buildAudienceFromWhere(f)
	sqlStr := `SELECT COUNT(*) ` + fromWhere
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, sqlStr, args...); err != nil {
		return 0, err
	}
	return total, nil
}

func (m *defaultRuleQueryModel) ListAudience(ctx context.Context, f AudienceFilter, size, offset int64) ([]AudienceUserRow, error) {
	fromWhere, args := m.buildAudienceFromWhere(f)
	// LIMIT/OFFSET 作为最后两个参数
	sqlStr := `SELECT u.user_id::text AS user_id, COALESCE(u.username, u.user_id::text) AS username ` + fromWhere +
		fmt.Sprintf(` ORDER BY u.user_id ASC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	listArgs := append(args, size, offset)
	var rows []AudienceUserRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, sqlStr, listArgs...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *defaultRuleQueryModel) ListAudienceUserIds(ctx context.Context, f AudienceFilter) ([]string, error) {
	fromWhere, args := m.buildAudienceFromWhere(f)
	sqlStr := `SELECT u.user_id::text AS user_id ` + fromWhere
	var userIds []string
	if err := m.conn.QueryRowsCtx(ctx, &userIds, sqlStr, args...); err != nil {
		return nil, err
	}
	return userIds, nil
}

