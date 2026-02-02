package users

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/stores/builder"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/stringx"
)

var (
	aeRechargeAllocationFieldNames          = builder.RawFieldNames(&AeRechargeAllocation{}, true)
	aeRechargeAllocationRows                = strings.Join(aeRechargeAllocationFieldNames, ",")
	aeRechargeAllocationRowsExpectAutoSet   = strings.Join(stringx.Remove(aeRechargeAllocationFieldNames, "id", "create_time"), ",")
	aeRechargeAllocationRowsWithPlaceHolder = builder.PostgreSqlJoin(stringx.Remove(aeRechargeAllocationFieldNames, "id", "create_time"))
)

type (
	aeRechargeAllocationModel interface {
		Insert(ctx context.Context, data *AeRechargeAllocation) (sql.Result, error)
		FindOne(ctx context.Context, id int64) (*AeRechargeAllocation, error)
		Update(ctx context.Context, data *AeRechargeAllocation) error
		Delete(ctx context.Context, id int64) error
		TableName() string
	}

	defaultAeRechargeAllocationModel struct {
		conn  sqlx.SqlConn
		table string
	}

	AeRechargeAllocation struct {
		Id                 int64           `db:"id"`
		ConsumptionDailyId int64           `db:"consumption_daily_id"`
		RechargeRecordId   int64           `db:"recharge_record_id"`
		DeductedAmount     decimal.Decimal `db:"deducted_amount"`
		CreateTime         time.Time       `db:"create_time"`
	}
)

func newAeRechargeAllocationModel(conn sqlx.SqlConn) *defaultAeRechargeAllocationModel {
	return &defaultAeRechargeAllocationModel{
		conn:  conn,
		table: `"public"."ae_recharge_allocation"`,
	}
}

func (m *defaultAeRechargeAllocationModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf("delete from %s where id = $1", m.table)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

func (m *defaultAeRechargeAllocationModel) FindOne(ctx context.Context, id int64) (*AeRechargeAllocation, error) {
	query := fmt.Sprintf("select %s from %s where id = $1 limit 1", aeRechargeAllocationRows, m.table)
	var resp AeRechargeAllocation
	err := m.conn.QueryRowCtx(ctx, &resp, query, id)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultAeRechargeAllocationModel) Insert(ctx context.Context, data *AeRechargeAllocation) (sql.Result, error) {
	query := fmt.Sprintf("insert into %s (%s) values (%s)", m.table, aeRechargeAllocationRowsExpectAutoSet, aeRechargeAllocationRowsWithPlaceHolder)
	ret, err := m.conn.ExecCtx(ctx, query, data.ConsumptionDailyId, data.RechargeRecordId, data.DeductedAmount)
	return ret, err
}

func (m *defaultAeRechargeAllocationModel) Update(ctx context.Context, data *AeRechargeAllocation) error {
	query := fmt.Sprintf("update %s set %s where id = $1", m.table, "consumption_daily_id=$2, recharge_record_id=$3, deducted_amount=$4")
	_, err := m.conn.ExecCtx(ctx, query, data.Id, data.ConsumptionDailyId, data.RechargeRecordId, data.DeductedAmount)
	return err
}

func (m *defaultAeRechargeAllocationModel) TableName() string {
	return m.table
}
