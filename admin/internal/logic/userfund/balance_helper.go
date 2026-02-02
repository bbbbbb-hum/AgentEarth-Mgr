package userfund

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
)

// Queryable 定义了 sqlx.SqlConn 和 sqlx.Session 都满足的接口
// 用于在事务和非事务场景下复用查询逻辑
type Queryable interface {
	QueryRowCtx(ctx context.Context, v interface{}, query string, args ...interface{}) error
}

// CalculateRealTimeBalance 计算特定充值记录的实时余额
// 实现逻辑：当前余额 = initialAmount - 消费扣减总和(ae_recharge_allocation) - 系统扣减总和(ae_user_recharge_record负值)
func CalculateRealTimeBalance(ctx context.Context, conn Queryable, recordID int64, initialAmount decimal.Decimal) (decimal.Decimal, error) {
	// 使用高效的子查询直接在数据库层完成计算，利用数据库聚合能力
	query := `
		SELECT
			$2::numeric
			- COALESCE((SELECT SUM(deducted_amount) FROM ae_recharge_allocation WHERE recharge_record_id = $1), 0)
			- COALESCE((SELECT SUM(ABS(xlcredit_amount)) FROM ae_user_recharge_record WHERE related_parent_id = $1 AND xlcredit_amount < 0), 0)
	`

	// 使用 string 承接 numeric，避免驱动/框架对 Decimal 的扫描不兼容
	var currentBalanceStr string
	err := conn.QueryRowCtx(ctx, &currentBalanceStr, query, recordID, initialAmount.String())
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to calculate real-time balance for record %d: %w", recordID, err)
	}

	currentBalance, err := decimal.NewFromString(currentBalanceStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to parse balance for record %d: %w", recordID, err)
	}

	return currentBalance, nil
}
