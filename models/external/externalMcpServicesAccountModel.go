package external

import (
	"AgentEarth-Mgr/models"
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ExternalMcpServicesAccountModel = (*customExternalMcpServicesAccountModel)(nil)

type (
	// ExternalMcpServicesAccountModel is an interface to be customized, add more methods here,
	// and implement the added methods in customExternalMcpServicesAccountModel.
	ExternalMcpServicesAccountModel interface {
		externalMcpServicesAccountModel
		withSession(session sqlx.Session) ExternalMcpServicesAccountModel
		GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*ExternalMcpServicesAccount, total int64, err error)
	}

	customExternalMcpServicesAccountModel struct {
		*defaultExternalMcpServicesAccountModel
	}
)

// NewExternalMcpServicesAccountModel returns a model for the database table.
func NewExternalMcpServicesAccountModel(conn sqlx.SqlConn) ExternalMcpServicesAccountModel {
	return &customExternalMcpServicesAccountModel{
		defaultExternalMcpServicesAccountModel: newExternalMcpServicesAccountModel(conn),
	}
}

func (m *customExternalMcpServicesAccountModel) withSession(session sqlx.Session) ExternalMcpServicesAccountModel {
	return NewExternalMcpServicesAccountModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customExternalMcpServicesAccountModel) GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*ExternalMcpServicesAccount, total int64, err error) {
	countQuery := fmt.Sprintf("select count(*) as number from %s", m.table)
	query := fmt.Sprintf("select %s from %s", externalMcpServicesAccountRows, m.table)
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
