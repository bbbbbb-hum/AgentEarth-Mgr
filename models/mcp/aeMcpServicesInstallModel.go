package mcp

import (
	"AgentEarth-Mgr/models"
	"context"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeMcpServicesInstallModel = (*customAeMcpServicesInstallModel)(nil)

type (
	// AeMcpServicesInstallModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeMcpServicesInstallModel.
	AeMcpServicesInstallModel interface {
		aeMcpServicesInstallModel
		withSession(session sqlx.Session) AeMcpServicesInstallModel
		FindOneByCondition(ctx context.Context, conditions []models.Condition) (*AeMcpServicesInstall, error)
		GetMaxPort(ctx context.Context) (int64, error)
		GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpServicesInstall, total int64, err error)
		DeleteByConditions(ctx context.Context, conditions []models.Condition) error
	}

	customAeMcpServicesInstallModel struct {
		*defaultAeMcpServicesInstallModel
	}
)

// NewAeMcpServicesInstallModel returns a model for the database table.
func NewAeMcpServicesInstallModel(conn sqlx.SqlConn) AeMcpServicesInstallModel {
	return &customAeMcpServicesInstallModel{
		defaultAeMcpServicesInstallModel: newAeMcpServicesInstallModel(conn),
	}
}

func (m *customAeMcpServicesInstallModel) withSession(session sqlx.Session) AeMcpServicesInstallModel {
	return NewAeMcpServicesInstallModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAeMcpServicesInstallModel) FindOneByCondition(ctx context.Context, conditions []models.Condition) (*AeMcpServicesInstall, error) {
	query := fmt.Sprintf("select %s from %s", aeMcpServicesInstallRows, m.table)
	//处理where条件
	whereClause, args, err1 := models.DealWithWhereSafe(conditions...)
	if err1 != nil {
		return nil, err1
	}
	query += whereClause
	query += " limit 1"
	var resp AeMcpServicesInstall
	err := m.conn.QueryRowCtx(ctx, &resp, query, args...)
	switch {
	case err == nil:
		return &resp, nil
	case errors.Is(err, sqlx.ErrNotFound):
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customAeMcpServicesInstallModel) GetMaxPort(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("select COALESCE(MAX(port), 0) as max_port from %s", m.table)
	var maxPort int64
	err := m.conn.QueryRowCtx(ctx, &maxPort, query)
	if err != nil {
		return 0, err
	}
	return maxPort, nil
}

func (m *customAeMcpServicesInstallModel) GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpServicesInstall, total int64, err error) {
	countQuery := fmt.Sprintf("select count(*) as number from %s", m.table)
	query := fmt.Sprintf("select %s from %s", aeMcpServicesInstallRows, m.table)
	//lp.Conditions = append(lp.Conditions, models.Condition{
	//	Field: "enabled",
	//	Value: true,
	//})
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

func (m *customAeMcpServicesInstallModel) DeleteByConditions(ctx context.Context, conditions []models.Condition) error {
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
