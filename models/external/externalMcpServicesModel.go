package external

import (
	"AgentEarth-Mgr/models"
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ExternalMcpServicesModel = (*customExternalMcpServicesModel)(nil)

type (
	// ExternalMcpServicesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customExternalMcpServicesModel.
	ExternalMcpServicesModel interface {
		externalMcpServicesModel
		withSession(session sqlx.Session) ExternalMcpServicesModel
		GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*ExternalMcpServices, total int64, err error)
		UpdateTestStatus(ctx context.Context, ids []int64, status int64) error
		BatchUpdateTestStatus(ctx context.Context, ids []int64, status int64) error
		UpsertWithId(ctx context.Context, data *ExternalMcpServices) error
	}

	customExternalMcpServicesModel struct {
		*defaultExternalMcpServicesModel
	}
)

// NewExternalMcpServicesModel returns a model for the database table.
func NewExternalMcpServicesModel(conn sqlx.SqlConn) ExternalMcpServicesModel {
	return &customExternalMcpServicesModel{
		defaultExternalMcpServicesModel: newExternalMcpServicesModel(conn),
	}
}

func (m *customExternalMcpServicesModel) withSession(session sqlx.Session) ExternalMcpServicesModel {
	return NewExternalMcpServicesModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customExternalMcpServicesModel) GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*ExternalMcpServices, total int64, err error) {
	countQuery := fmt.Sprintf("select count(*) as number from %s", m.table)
	query := fmt.Sprintf("select %s from %s", externalMcpServicesRows, m.table)
	//处理where条件
	whereClause, args, err := models.DealWithWhereSafe(lp.Conditions...)
	if err != nil {
		return
	}
	countQuery += whereClause
	query += whereClause
	var totals []models.Total
	err = m.conn.QueryRowsCtx(ctx, &totals, countQuery, args...)
	if err != nil {
		return
	}
	total = totals[0].Number
	if total == 0 {
		return
	}
	if getList {
		//排序
		if len(lp.Sorts) > 0 && len(lp.Sorts[0].Filed) > 0 && len(lp.Sorts[0].Order) > 0 {
			query = models.GetOrderBy(lp.Sorts, query)
		} else {
			query += " order by id desc"
		}
		if lp.Page > 0 && lp.Size > 0 {
			var offset = (lp.Page - 1) * lp.Size
			query += fmt.Sprintf(" limit %d offset %d", lp.Size, offset)
		}
		err = m.conn.QueryRowsCtx(ctx, &list, query, args...)
	}
	return
}

func (m *customExternalMcpServicesModel) UpdateTestStatus(ctx context.Context, ids []int64, status int64) error {
	// keep compatibility with logic layer naming; reuse the existing batch implementation
	return m.BatchUpdateTestStatus(ctx, ids, status)
}

func (m *customExternalMcpServicesModel) BatchUpdateTestStatus(ctx context.Context, ids []int64, status int64) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, 1+len(ids))
	args = append(args, status)
	for i, id := range ids {
		// status is $1, ids start from $2
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2))
		args = append(args, id)
	}

	query := fmt.Sprintf("update %s set test_status = $1 where id in (%s)", m.table, strings.Join(placeholders, ","))
	_, err := m.conn.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	return nil
}

func (m *customExternalMcpServicesModel) UpsertWithId(ctx context.Context, data *ExternalMcpServices) error {
	// Insert all fields (including id) and update all non-id fields on conflict.
	placeholders := make([]string, 0, len(externalMcpServicesFieldNames))
	for i := range externalMcpServicesFieldNames {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	updateCols := make([]string, 0, len(externalMcpServicesFieldNames)-1)
	for _, col := range externalMcpServicesFieldNames {
		if col == "id" {
			continue
		}
		updateCols = append(updateCols, fmt.Sprintf("%s = excluded.%s", col, col))
	}

	query := fmt.Sprintf(
		"insert into %s (%s) values (%s) on conflict (id) do update set %s",
		m.table,
		externalMcpServicesRows,
		strings.Join(placeholders, ","),
		strings.Join(updateCols, ","),
	)

	_, err := m.conn.ExecCtx(ctx, query,
		data.Id,
		data.ServerName,
		data.ServerType,
		data.LaunchInfo,
		data.ConnectInfo,
		data.Description,
		data.CreateTime,
		data.UpdateTime,
		data.CodeSourceUrl,
		data.TestStatus,
		data.DockerCmd,
		data.ProjectName,
		data.Valuable,
		data.Calls,
		data.Category,
		data.Image,
		data.CloneRepository,
		data.NeedKey,
		data.Install,
	)
	return err
}
