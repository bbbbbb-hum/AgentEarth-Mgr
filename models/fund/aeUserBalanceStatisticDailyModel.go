package fund

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AeUserBalanceStatisticDailyModel 用于访问 ae_user_balance_statistic_daily 用户日余额表
// 只提供查询最近一条余额数据的方法，统计写入由其他系统负责。
type AeUserBalanceStatisticDailyModel interface {
	GetLatestBalance(ctx context.Context, userId string) (float64, error)
	QueryLatestBalanceSnapshot(ctx context.Context, userId string) (*BalanceSnapshot, error)
	QueryBalanceHistory(ctx context.Context, userId string, days int64) ([]BalanceHistoryRow, error)
	WithSession(session sqlx.Session) AeUserBalanceStatisticDailyModel
}

// customAeUserBalanceStatisticDailyModel 提供余额统计的只读查询能力，独立于 goctl 生成的 default 模型。
type customAeUserBalanceStatisticDailyModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewAeUserBalanceStatisticDailyModel(conn sqlx.SqlConn) AeUserBalanceStatisticDailyModel {
	return &customAeUserBalanceStatisticDailyModel{
		conn:  conn,
		table: `"public"."ae_user_balance_statistic_daily"`,
	}
}

func (m *customAeUserBalanceStatisticDailyModel) WithSession(session sqlx.Session) AeUserBalanceStatisticDailyModel {
	return &customAeUserBalanceStatisticDailyModel{
		conn:  sqlx.NewSqlConnFromSession(session),
		table: m.table,
	}
}

// GetLatestBalance 获取某个用户在余额统计表中最近一天的余额
// 如果没有记录，则返回 0，不视为错误。
func (m *customAeUserBalanceStatisticDailyModel) GetLatestBalance(ctx context.Context, userId string) (float64, error) {
	query := fmt.Sprintf(`
		select balance
		from %s
		where user_id = $1
		order by day desc
		limit 1
	`, m.table)

	var balance float64
	err := m.conn.QueryRowCtx(ctx, &balance, query, userId)
	switch err {
	case nil: //查询成功，且至少有一行
		return balance, nil
	case sqlx.ErrNotFound: //查询成功，但没有查到任何一行
		return 0, nil
	default: //真正出现了错误
		return 0, err
	}
}

// BalanceSnapshot 表示 ae_user_balance_statistic_daily 中最近一条余额快照。
// - Balance 为 numeric 类型的字符串表示，上层可用 decimal 解析。
// - Day 为快照对应的统计日期。
type BalanceSnapshot struct {
	Balance string    `db:"balance"`
	Day     time.Time `db:"day"`
}

// QueryLatestBalanceSnapshot 查询用户在日余额统计表中的最新快照。
// 返回值：
// - (*BalanceSnapshot, nil): 查询成功且存在记录；
// - (nil, nil): 查无记录（无快照，以 0 余额视之）；
// - (nil, err): 查询过程中发生错误。
func (m *customAeUserBalanceStatisticDailyModel) QueryLatestBalanceSnapshot(ctx context.Context, userId string) (*BalanceSnapshot, error) {
	const snapshotQuery = `
		SELECT balance, day
		FROM ae_user_balance_statistic_daily
		WHERE user_id = $1
		ORDER BY day DESC
		LIMIT 1
	`
	var snap BalanceSnapshot
	if err := m.conn.QueryRowCtx(ctx, &snap, snapshotQuery, userId); err != nil {
		if errors.Is(err, sql.ErrNoRows) || err == sqlx.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &snap, nil
}

// BalanceHistoryRow 用于“余额趋势曲线图”的数据载体。
// - Day: 日期（YYYY-MM-DD 字符串）
// - Balance: 该日展示余额，使用“向前填充”策略处理缺失日期。
type BalanceHistoryRow struct {
	Day     string  `db:"day"`
	Balance float64 `db:"balance"`
}

// QueryBalanceHistory 查询指定用户最近 N 天（含今天）的余额历史。
// 实现要点：
//   - 通过 date_series 生成完整日期序列，确保每天都有一条记录。
//   - 对于某天没有统计记录的情况，从 ae_user_balance_statistic_daily 中取
//     “该用户 <= 当天 的最近一条余额”，实现向前填充。
func (m *customAeUserBalanceStatisticDailyModel) QueryBalanceHistory(ctx context.Context, userId string, days int64) ([]BalanceHistoryRow, error) {
	//generate_series(...)是pgsql的一个内置函数，可以生成一个序列(数字/时间/日期序列),generate_series(起始值，结束值，步长)
	//用一个公用表表达式(CTE),可以理解为是一个临时虚拟表
	//用generate_series保证日期不中断，用order by...limit 1的子查询来实现“如果没有今天的记录，就沿用最近一次的余额”
	const query = `
		WITH date_series AS (
			SELECT generate_series(
				CURRENT_DATE - INTERVAL '1 day' * ($2 - 1),
				CURRENT_DATE,
				INTERVAL '1 day'
			)::date AS day
		)
		SELECT ds.day::text as day,
			COALESCE((
				SELECT s.balance FROM ae_user_balance_statistic_daily s
				WHERE s.user_id = $1 AND s.day <= ds.day
				ORDER BY s.day DESC LIMIT 1
			), 0) as balance
		FROM date_series ds
		ORDER BY ds.day ASC
	`
	var rows []BalanceHistoryRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, userId, days); err != nil {
		return nil, err
	}
	return rows, nil
}
