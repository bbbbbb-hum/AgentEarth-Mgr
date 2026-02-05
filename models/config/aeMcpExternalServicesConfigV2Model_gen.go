package config

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/stores/builder"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/stringx"
)

var (
	aeMcpExternalServicesConfigV2FieldNames          = builder.RawFieldNames(&AeMcpExternalServicesConfigV2{}, true)
	aeMcpExternalServicesConfigV2Rows                = strings.Join(aeMcpExternalServicesConfigV2FieldNames, ",")
	aeMcpExternalServicesConfigV2RowsExpectAutoSet   = strings.Join(stringx.Remove(aeMcpExternalServicesConfigV2FieldNames, "id", "create_at", "create_time", "created_at", "update_at", "update_time", "updated_at"), ",")
	aeMcpExternalServicesConfigV2RowsWithPlaceHolder = builder.PostgreSqlJoin(stringx.Remove(aeMcpExternalServicesConfigV2FieldNames, "id", "create_at", "create_time", "created_at", "update_at", "update_time", "updated_at"))
)

type (
	aeMcpExternalServicesConfigV2Model interface {
		Insert(ctx context.Context, data *AeMcpExternalServicesConfigV2) (sql.Result, error)
		FindOne(ctx context.Context, id int64) (*AeMcpExternalServicesConfigV2, error)
		Update(ctx context.Context, data *AeMcpExternalServicesConfigV2) error
		Delete(ctx context.Context, id int64) error
	}

	defaultAeMcpExternalServicesConfigV2Model struct {
		conn  sqlx.SqlConn
		table string
	}

	AeMcpExternalServicesConfigV2 struct {
		Id              int64          `db:"id"`
		Name            string         `db:"name"`
		WemcpName       string         `db:"wemcp_name"`
		Tags            pq.StringArray `db:"tags"`
		Description     string         `db:"description"`
		Comments        string         `db:"comments"`
		CodeSourceUrl   string         `db:"code_source_url"`
		AccountRequired int64          `db:"account_required"`
		TestStatus      int64          `db:"test_status"`
		OnlineStatus    int64          `db:"online_status"`
		CreateTime      time.Time      `db:"create_time"`
		UpdateTime      time.Time      `db:"update_time"`
	}
)

func newAeMcpExternalServicesConfigV2Model(conn sqlx.SqlConn) *defaultAeMcpExternalServicesConfigV2Model {
	return &defaultAeMcpExternalServicesConfigV2Model{
		conn:  conn,
		table: `"public"."ae_mcp_external_services_config_v2"`,
	}
}

func (m *defaultAeMcpExternalServicesConfigV2Model) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf("delete from %s where id = $1", m.table)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

func (m *defaultAeMcpExternalServicesConfigV2Model) FindOne(ctx context.Context, id int64) (*AeMcpExternalServicesConfigV2, error) {
	query := fmt.Sprintf("select %s from %s where id = $1 limit 1", aeMcpExternalServicesConfigV2Rows, m.table)
	var resp AeMcpExternalServicesConfigV2
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

func (m *defaultAeMcpExternalServicesConfigV2Model) Insert(ctx context.Context, data *AeMcpExternalServicesConfigV2) (sql.Result, error) {
	query := fmt.Sprintf("insert into %s (%s) values ($1, $2, $3, $4, $5, $6, $7, $8, $9)", m.table, aeMcpExternalServicesConfigV2RowsExpectAutoSet)
	ret, err := m.conn.ExecCtx(ctx, query, data.Name, data.WemcpName, data.Tags, data.Description, data.Comments, data.CodeSourceUrl, data.AccountRequired, data.TestStatus, data.OnlineStatus)
	return ret, err
}

func (m *defaultAeMcpExternalServicesConfigV2Model) Update(ctx context.Context, data *AeMcpExternalServicesConfigV2) error {
	query := fmt.Sprintf("update %s set %s where id = $1", m.table, aeMcpExternalServicesConfigV2RowsWithPlaceHolder)
	_, err := m.conn.ExecCtx(ctx, query, data.Id, data.Name, data.WemcpName, data.Tags, data.Description, data.Comments, data.CodeSourceUrl, data.AccountRequired, data.TestStatus, data.OnlineStatus)
	return err
}

func (m *defaultAeMcpExternalServicesConfigV2Model) tableName() string {
	return m.table
}

