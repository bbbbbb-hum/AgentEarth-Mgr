package users

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AeRechargeAllocationModel = (*customAeRechargeAllocationModel)(nil)

type (
	// AeRechargeAllocationModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeRechargeAllocationModel.
	AeRechargeAllocationModel interface {
		aeRechargeAllocationModel
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
