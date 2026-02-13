package config

import (
	"AgentEarth-Mgr/models"
	"context"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeMcpTaskNodeModel = (*customAeMcpTaskNodeModel)(nil)

type (
	// AeMcpTaskNodeModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeMcpTaskNodeModel.
	AeMcpTaskNodeModel interface {
		aeMcpTaskNodeModel
		withSession(session sqlx.Session) AeMcpTaskNodeModel
		// InsertReturningId inserts and returns id (PostgreSQL RETURNING)
		InsertReturningId(ctx context.Context, data *AeMcpTaskNode) (int64, error)
		GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpTaskNode, total int64, err error)
		DeleteByConditions(ctx context.Context, conditions []models.Condition) error
	}

	customAeMcpTaskNodeModel struct {
		*defaultAeMcpTaskNodeModel
	}
)

// NewAeMcpTaskNodeModel returns a model for the database table.
func NewAeMcpTaskNodeModel(conn sqlx.SqlConn) AeMcpTaskNodeModel {
	return &customAeMcpTaskNodeModel{
		defaultAeMcpTaskNodeModel: newAeMcpTaskNodeModel(conn),
	}
}

func (m *customAeMcpTaskNodeModel) withSession(session sqlx.Session) AeMcpTaskNodeModel {
	return NewAeMcpTaskNodeModel(sqlx.NewSqlConnFromSession(session))
}

// InsertReturningId inserts a record and returns the generated id using RETURNING.
func (m *customAeMcpTaskNodeModel) InsertReturningId(ctx context.Context, data *AeMcpTaskNode) (int64, error) {
	query := fmt.Sprintf("insert into %s (%s) values ($1, $2, $3, $4, $5, $6) returning id", m.table, aeMcpTaskNodeRowsExpectAutoSet)
	var id int64
	if err := m.conn.QueryRowCtx(ctx, &id, query, data.NodeName, data.NodeHandle, data.Enabled, data.Description, data.ServerId, data.NodeConfig); err != nil {
		return 0, err
	}
	return id, nil
}

func (m *customAeMcpTaskNodeModel) GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpTaskNode, total int64, err error) {
	countQuery := fmt.Sprintf("select count(*) as number from %s", m.table)
	query := fmt.Sprintf("select %s from %s", aeMcpTaskNodeRows, m.table)
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

func (m *customAeMcpTaskNodeModel) DeleteByConditions(ctx context.Context, conditions []models.Condition) error {
	query := fmt.Sprintf("delete from %s", m.table)
	//处理where条件
	whereClause, args, err1 := models.DealWithWhereSafe(conditions...)
	if err1 != nil {
		return err1
	}
	if len(whereClause) == 0 {
		return errors.New("条件不能为空")
	}
	if len(args) == 0 {
		return errors.New("参数不能为空")
	}
	query += whereClause
	_, err := m.conn.ExecCtx(ctx, query, args...)
	return err
}
