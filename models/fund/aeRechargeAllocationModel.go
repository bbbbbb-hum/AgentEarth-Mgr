package fund

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeRechargeAllocationModel = (*customAeRechargeAllocationModel)(nil)

type (
	// AeRechargeAllocationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeRechargeAllocationModel.
	AeRechargeAllocationModel interface {
		aeRechargeAllocationModel

		// GetAllocatedSumByRechargeChildren 查询某个正向充值批次“子记录”上的已核销金额总和。
		// 业务含义：
		// - 过期扣减/管理员扣减等负值记录会通过 related_recharge_id 指向原始正向批次；
		// - 若在这些负值记录上继续挂载 allocation（例如过期后仍发生消费透支），
		//   则这些 allocation 也应被视为原始批次的“总流出”一部分，用于计算透支金额。
		// SQL 语义：
		//   SUM(a.deducted_amount) WHERE a.recharge_record_id IN (
		//     SELECT id FROM ae_user_recharge_record
		//     WHERE related_recharge_id = $rechargeID AND xlcredit_amount < 0
		//   )
		// 返回 numeric 字符串，便于上层用 decimal 做精度运算。
		GetAllocatedSumByRechargeChildren(ctx context.Context, rechargeID int64) (string, error)
	}

	customAeRechargeAllocationModel struct {
		*defaultAeRechargeAllocationModel
	}
)

// NewAeRechargeAllocationModel returns a model for the database table.
func NewAeRechargeAllocationModel(conn sqlx.SqlConn) AeRechargeAllocationModel {
	return &customAeRechargeAllocationModel{
		defaultAeRechargeAllocationModel: newAeRechargeAllocationModel(conn),
	}
}

// GetAllocatedSumByRechargeChildren 实现见接口注释。
func (m *customAeRechargeAllocationModel) GetAllocatedSumByRechargeChildren(ctx context.Context, rechargeID int64) (string, error) {
	const query = `
		SELECT COALESCE(SUM(a.deducted_amount), 0)::text
		FROM ae_recharge_allocation a
		JOIN ae_user_recharge_record r_child ON a.recharge_record_id = r_child.id
		WHERE r_child.related_recharge_id = $1 AND r_child.xlcredit_amount < 0
	`
	var sumStr string
	if err := m.conn.QueryRowCtx(ctx, &sumStr, query, rechargeID); err != nil {
		return "", err
	}
	return sumStr, nil
}

