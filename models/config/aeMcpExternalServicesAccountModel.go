package config

import (
	"AgentEarth-Mgr/models"
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeMcpExternalServicesAccountModel = (*customAeMcpExternalServicesAccountModel)(nil)

type (
	// AeMcpExternalServicesAccountModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeMcpExternalServicesAccountModel.
	AeMcpExternalServicesAccountModel interface {
		aeMcpExternalServicesAccountModel
		withSession(session sqlx.Session) AeMcpExternalServicesAccountModel
		GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpExternalServicesAccount, total int64, err error)
		UpdateStatus(ctx context.Context, id int64, status string) error
		BatchUpdateStatus(ctx context.Context, ids []int64, status string) error
		BatchDelete(ctx context.Context, ids []int64) error
	}

	customAeMcpExternalServicesAccountModel struct {
		*defaultAeMcpExternalServicesAccountModel
	}
)

// NewAeMcpExternalServicesAccountModel returns a model for the database table.
func NewAeMcpExternalServicesAccountModel(conn sqlx.SqlConn) AeMcpExternalServicesAccountModel {
	return &customAeMcpExternalServicesAccountModel{
		defaultAeMcpExternalServicesAccountModel: newAeMcpExternalServicesAccountModel(conn),
	}
}

func (m *customAeMcpExternalServicesAccountModel) withSession(session sqlx.Session) AeMcpExternalServicesAccountModel {
	return NewAeMcpExternalServicesAccountModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAeMcpExternalServicesAccountModel) GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpExternalServicesAccount, total int64, err error) {
	countQuery := fmt.Sprintf("select count(*) as number from %s", m.table)
	query := fmt.Sprintf("select %s from %s", aeMcpExternalServicesAccountRows, m.table)
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

func (m *customAeMcpExternalServicesAccountModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := fmt.Sprintf("update %s set status = $1 where id = $2", m.table)
	ret, err := m.conn.ExecCtx(ctx, query, status, id)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *customAeMcpExternalServicesAccountModel) BatchUpdateStatus(ctx context.Context, ids []int64, status string) error {
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

	query := fmt.Sprintf("update %s set status = $1 where id in (%s)", m.table, strings.Join(placeholders, ","))
	ret, err := m.conn.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *customAeMcpExternalServicesAccountModel) BatchDelete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for i, id := range ids {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}

	query := fmt.Sprintf("delete from %s where id in (%s)", m.table, strings.Join(placeholders, ","))
	ret, err := m.conn.ExecCtx(ctx, query, args...)
	if err != nil {
		return err
	}
	affected, err := ret.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
