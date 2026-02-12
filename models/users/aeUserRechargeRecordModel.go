package users

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeUserRechargeRecordModel = (*customAeUserRechargeRecordModel)(nil)

type (
	// AeUserRechargeRecordModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeUserRechargeRecordModel.
	AeUserRechargeRecordModel interface {
		aeUserRechargeRecordModel
		withSession(session sqlx.Session) AeUserRechargeRecordModel
		Sum24hRecharge(ctx context.Context) (float64, error)
		GetBalance(ctx context.Context, userId string) (float64, error)
	}

	customAeUserRechargeRecordModel struct {
		*defaultAeUserRechargeRecordModel
	}
)

// NewAeUserRechargeRecordModel returns a model for the database table.
func NewAeUserRechargeRecordModel(conn sqlx.SqlConn) AeUserRechargeRecordModel {
	return &customAeUserRechargeRecordModel{
		defaultAeUserRechargeRecordModel: newAeUserRechargeRecordModel(conn),
	}
}

func (m *customAeUserRechargeRecordModel) withSession(session sqlx.Session) AeUserRechargeRecordModel {
	return NewAeUserRechargeRecordModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAeUserRechargeRecordModel) Sum24hRecharge(ctx context.Context) (float64, error) {
	// 统计24小时内所有用户充值的总和
	// 使用 pay_time 字段，只统计正值（充值），排除负值（扣减）
	// xlcredit_amount > 0 表示充值，< 0 表示扣减
	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(xlcredit_amount), 0) 
		FROM %s 
		WHERE pay_time >= NOW() - INTERVAL '24 HOURS' 
		AND xlcredit_amount > 0
	`, m.table)
	var total float64
	err := m.conn.QueryRowCtx(ctx, &total, query)
	return total, err
}

func (m *customAeUserRechargeRecordModel) GetBalance(ctx context.Context, userId string) (float64, error) {
	query := fmt.Sprintf("select COALESCE(SUM(xlcredit_amount), 0) from %s where user_id = $1", m.table)
	var balance float64
	err := m.conn.QueryRowCtx(ctx, &balance, query, userId)
	return balance, err
}
