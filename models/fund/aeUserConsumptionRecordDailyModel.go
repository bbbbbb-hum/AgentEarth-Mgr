package fund

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeUserConsumptionRecordDailyModel = (*customAeUserConsumptionRecordDailyModel)(nil)

type (
	// AeUserConsumptionRecordDailyModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeUserConsumptionRecordDailyModel.
	AeUserConsumptionRecordDailyModel interface {
		aeUserConsumptionRecordDailyModel
		WithSession(session sqlx.Session) AeUserConsumptionRecordDailyModel
		QueryConsumptionRecords(ctx context.Context, userId string, days int64) ([]ConsumptionRecordRow, error)
		GetAvgDailyConsumptionLast30Days(ctx context.Context, userId string) (float64, error)
	}

	customAeUserConsumptionRecordDailyModel struct {
		*defaultAeUserConsumptionRecordDailyModel
	}
)

// NewAeUserConsumptionRecordDailyModel returns a model for the database table.
func NewAeUserConsumptionRecordDailyModel(conn sqlx.SqlConn) AeUserConsumptionRecordDailyModel {
	return &customAeUserConsumptionRecordDailyModel{
		defaultAeUserConsumptionRecordDailyModel: newAeUserConsumptionRecordDailyModel(conn),
	}
}

func (m *customAeUserConsumptionRecordDailyModel) WithSession(session sqlx.Session) AeUserConsumptionRecordDailyModel {
	return NewAeUserConsumptionRecordDailyModel(sqlx.NewSqlConnFromSession(session))
}

// ConsumptionRecordRow 用于“最近 N 天消费/系统扣减堆叠柱状图”的数据载体。
// - self_consume: 用户自行消费（ae_user_consumption_record_daily.xlcredit_consume > 0）
// - system_deduct: 系统/后台扣减（来自 ae_user_recharge_record 的负值汇总）
type ConsumptionRecordRow struct {
	Day          string  `db:"day"`
	SelfConsume  float64 `db:"self_consume"`
	SystemDeduct float64 `db:"system_deduct"`
}

// QueryConsumptionRecords 查询指定用户最近 N 天（含今天）的每日消费与系统扣减。
// SQL 说明：
// - 使用 date_series 生成完整日期序列，保证缺失日期也会返回一行，方便前端绘制连续柱状图。
// - consume_daily：聚合 ae_user_consumption_record_daily 中的正向消费。
// - recharge_deduction_daily：聚合 ae_user_recharge_record 中的负值扣减（按 pay_time::date 维度）。
// - 最终 LEFT JOIN 两个子查询，对缺失数据使用 COALESCE 补 0。
func (m *customAeUserConsumptionRecordDailyModel) QueryConsumptionRecords(ctx context.Context, userId string, days int64) ([]ConsumptionRecordRow, error) {
	const query = `
		WITH date_series AS (
			SELECT generate_series(
				CURRENT_DATE - INTERVAL '1 day' * ($2 - 1),
				CURRENT_DATE,
				INTERVAL '1 day'
			)::date AS day
		),
		consume_daily AS (
			SELECT day, COALESCE(SUM(xlcredit_consume), 0) AS self_consume
			FROM ae_user_consumption_record_daily
			WHERE user_id = $1 AND day >= CURRENT_DATE - INTERVAL '1 day' * ($2 - 1) AND day <= CURRENT_DATE
			GROUP BY day
		),
		recharge_deduction_daily AS (
			SELECT pay_time::date AS day, COALESCE(ABS(SUM(xlcredit_amount)), 0) AS system_deduct
			FROM ae_user_recharge_record
			WHERE user_id = $1 AND xlcredit_amount < 0
				AND pay_time::date >= CURRENT_DATE - INTERVAL '1 day' * ($2 - 1) AND pay_time::date <= CURRENT_DATE
			GROUP BY pay_time::date
		)
		SELECT ds.day::text as day, COALESCE(c.self_consume, 0) as self_consume, COALESCE(r.system_deduct, 0) as system_deduct
		FROM date_series ds
		LEFT JOIN consume_daily c ON c.day = ds.day
		LEFT JOIN recharge_deduction_daily r ON r.day = ds.day
		ORDER BY ds.day ASC
	`
	var rows []ConsumptionRecordRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, userId, days); err != nil {
		return nil, err
	}
	return rows, nil
}

// GetAvgDailyConsumptionLast30Days 统计用户最近 30 天的日均消费（只统计正向消费，xlcredit_consume > 0）。
// 说明：
// - 该查询在多个业务场景复用（用户列表、用户详情），因此下沉到 model 层统一维护。
func (m *customAeUserConsumptionRecordDailyModel) GetAvgDailyConsumptionLast30Days(ctx context.Context, userId string) (float64, error) {
	const query = `
		SELECT COALESCE(AVG(xlcredit_consume), 0) as avg_consumption
		FROM ae_user_consumption_record_daily
		WHERE user_id = $1 AND day >= CURRENT_DATE - INTERVAL '30 days' AND xlcredit_consume > 0
	`
	var avg float64
	if err := m.conn.QueryRowCtx(ctx, &avg, query, userId); err != nil {
		return 0, err
	}
	return avg, nil
}

