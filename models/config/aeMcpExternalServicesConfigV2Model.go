package config

import (
	"AgentEarth-Mgr/models"
	"context"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeMcpExternalServicesConfigV2Model = (*customAeMcpExternalServicesConfigV2Model)(nil)

type (
	AeMcpExternalServicesConfigV2Model interface {
		aeMcpExternalServicesConfigV2Model
		withSession(session sqlx.Session) AeMcpExternalServicesConfigV2Model
		GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpExternalServicesConfigV2, total int64, err error)
		FindOneByCondition(ctx context.Context, conditions []models.Condition) (*AeMcpExternalServicesConfigV2, error)
		DeleteByConditions(ctx context.Context, conditions []models.Condition) error
		BatchUpdateOnlineStatus(ctx context.Context, readyWemcpNames []string) error
	}

	customAeMcpExternalServicesConfigV2Model struct {
		*defaultAeMcpExternalServicesConfigV2Model
	}
)

func NewAeMcpExternalServicesConfigV2Model(conn sqlx.SqlConn) AeMcpExternalServicesConfigV2Model {
	return &customAeMcpExternalServicesConfigV2Model{
		defaultAeMcpExternalServicesConfigV2Model: newAeMcpExternalServicesConfigV2Model(conn),
	}
}

func (m *customAeMcpExternalServicesConfigV2Model) withSession(session sqlx.Session) AeMcpExternalServicesConfigV2Model {
	return NewAeMcpExternalServicesConfigV2Model(sqlx.NewSqlConnFromSession(session))
}

func (m *customAeMcpExternalServicesConfigV2Model) GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpExternalServicesConfigV2, total int64, err error) {
	countQuery := fmt.Sprintf("select count(*) as number from %s", m.table)
	query := fmt.Sprintf("select %s from %s", aeMcpExternalServicesConfigV2Rows, m.table)
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
		if len(lp.OrderBy) > 0 {
			query += " order by " + lp.OrderBy
		} else if len(lp.Sorts) > 0 && len(lp.Sorts[0].Filed) > 0 && len(lp.Sorts[0].Order) > 0 {
			query = models.GetOrderBy(lp.Sorts, query)
		} else {
			query += " order by id desc"
		}
		if lp.Page > 0 && lp.Size > 0 {
			offset := (lp.Page - 1) * lp.Size
			query += fmt.Sprintf(" limit %d offset %d", lp.Size, offset)
		}
		err = m.conn.QueryRowsCtx(ctx, &list, query, args...)
	}
	return
}

func (m *customAeMcpExternalServicesConfigV2Model) FindOneByCondition(ctx context.Context, conditions []models.Condition) (*AeMcpExternalServicesConfigV2, error) {
	query := fmt.Sprintf("select %s from %s", aeMcpExternalServicesConfigV2Rows, m.table)
	whereClause, args, err := models.DealWithWhereSafe(conditions...)
	if err != nil {
		return nil, err
	}
	query += whereClause
	query += " limit 1"
	var resp AeMcpExternalServicesConfigV2
	err = m.conn.QueryRowCtx(ctx, &resp, query, args...)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customAeMcpExternalServicesConfigV2Model) DeleteByConditions(ctx context.Context, conditions []models.Condition) error {
	query := fmt.Sprintf("delete from %s", m.table)
	whereClause, args, err := models.DealWithWhereSafe(conditions...)
	if err != nil {
		return err
	}
	if len(whereClause) == 0 {
		return errors.New("条件不能为空")
	}
	if len(args) == 0 {
		return errors.New("参数不能为空")
	}
	query += whereClause
	_, execErr := m.conn.ExecCtx(ctx, query, args...)
	return execErr
}

// BatchUpdateOnlineStatus 批量更新 online_status：
// readyWemcpNames 中的设为 1，其余设为 0，同时更新 update_time。
func (m *customAeMcpExternalServicesConfigV2Model) BatchUpdateOnlineStatus(ctx context.Context, readyWemcpNames []string) error {
	if len(readyWemcpNames) == 0 {
		// 没有就绪的服务，全部置 0
		query := fmt.Sprintf("UPDATE %s SET online_status = 0, update_time = NOW() WHERE online_status != 0", m.table)
		_, err := m.conn.ExecCtx(ctx, query)
		return err
	}

	// 将就绪的设为 1（仅当前值不为 1 时更新）
	query1 := fmt.Sprintf("UPDATE %s SET online_status = 1, update_time = NOW() WHERE wemcp_name = ANY($1) AND online_status != 1", m.table)
	if _, err := m.conn.ExecCtx(ctx, query1, pq.StringArray(readyWemcpNames)); err != nil {
		return err
	}

	// 将不在就绪列表中的设为 0（仅当前值不为 0 时更新）
	query2 := fmt.Sprintf("UPDATE %s SET online_status = 0, update_time = NOW() WHERE wemcp_name != ALL($1) AND online_status != 0", m.table)
	_, err := m.conn.ExecCtx(ctx, query2, pq.StringArray(readyWemcpNames))
	return err
}