package userfund

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
)

// Queryable 定义了 sqlx.SqlConn 和 sqlx.Session 都满足的接口
// 用于在事务和非事务场景下复用查询逻辑
// 在go-zero中，操作数据库主要通过两个不同的结构体：
// 1.*sqlx.DB:代表数据库连接池。通常用于普通的查询（非事务）
// 2.*sqlx.Tx:代表一个正在进行的事务，通常用于涉及资金变动的原子操作
// *sqlx.DB和*sqlx.Tx内部都实现了QueryRowCtx，在函数内部执行查询
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
			- COALESCE((SELECT SUM(ABS(xlcredit_amount)) FROM ae_user_recharge_record WHERE related_recharge_id = $1 AND xlcredit_amount < 0), 0)
	`

	// 使用 string 承接 numeric，避免驱动/框架对 Decimal 的扫描不兼容
	var currentBalanceStr string //指针类型的&currentBalanceStr去接受查询的返回值
	err := conn.QueryRowCtx(ctx, &currentBalanceStr, query, recordID, initialAmount.String())
	if err != nil {
		return decimal.Zero, fmt.Errorf("计算充值记录 %d 的实时余额失败: %w", recordID, err)
	}

	currentBalance, err := decimal.NewFromString(currentBalanceStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("解析充值记录 %d 的余额结果失败: %w", recordID, err)
	}

	return currentBalance, nil
}
