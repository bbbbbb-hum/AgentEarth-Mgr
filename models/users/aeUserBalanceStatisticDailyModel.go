package users

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AeUserBalanceStatisticDailyModel 用于访问 ae_user_balance_statistic_daily 用户日余额表
// 只提供查询最近一条余额数据的方法，统计写入由其他系统负责。
type AeUserBalanceStatisticDailyModel interface {
	GetLatestBalance(ctx context.Context, userId string) (float64, error)
}

type defaultAeUserBalanceStatisticDailyModel struct {
	conn  sqlx.SqlConn
	table string
}

func NewAeUserBalanceStatisticDailyModel(conn sqlx.SqlConn) AeUserBalanceStatisticDailyModel {
	return &defaultAeUserBalanceStatisticDailyModel{
		conn:  conn,
		table: `"public"."ae_user_balance_statistic_daily"`,
	}
}

// GetLatestBalance 获取某个用户在余额统计表中最近一天的余额
// 如果没有记录，则返回 0，不视为错误。
func (m *defaultAeUserBalanceStatisticDailyModel) GetLatestBalance(ctx context.Context, userId string) (float64, error) {
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
	case nil:
		return balance, nil
	case sqlx.ErrNotFound:
		// 没有记录时视为 0 余额
		return 0, nil
	default:
		return 0, err
	}
}

