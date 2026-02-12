package config

import (
	"AgentEarth-Mgr/models"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeMcpExternalServicesConfigModel = (*customAeMcpExternalServicesConfigModel)(nil)

type (
	// AeMcpExternalServicesConfigModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeMcpExternalServicesConfigModel.
	AeMcpExternalServicesConfigModel interface {
		aeMcpExternalServicesConfigModel
		withSession(session sqlx.Session) AeMcpExternalServicesConfigModel
		BatchInsert(ctx context.Context, list []AeMcpExternalServicesConfig) error
		GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpExternalServicesConfig, total int64, err error)
		FindOneByCondition(ctx context.Context, conditions []models.Condition) (*AeMcpExternalServicesConfig, error)
		DeleteByConditions(ctx context.Context, conditions []models.Condition) error
	}

	customAeMcpExternalServicesConfigModel struct {
		*defaultAeMcpExternalServicesConfigModel
	}
)

// NewAeMcpExternalServicesConfigModel returns a model for the database table.
func NewAeMcpExternalServicesConfigModel(conn sqlx.SqlConn) AeMcpExternalServicesConfigModel {
	return &customAeMcpExternalServicesConfigModel{
		defaultAeMcpExternalServicesConfigModel: newAeMcpExternalServicesConfigModel(conn),
	}
}

func (m *customAeMcpExternalServicesConfigModel) withSession(session sqlx.Session) AeMcpExternalServicesConfigModel {
	return NewAeMcpExternalServicesConfigModel(sqlx.NewSqlConnFromSession(session))
}

// BatchInsert inserts multiple records in one statement for performance.
func (m *customAeMcpExternalServicesConfigModel) BatchInsert(ctx context.Context, list []AeMcpExternalServicesConfig) error {
	if len(list) == 0 {
		return nil
	}
	columns := []string{"name", "type", "launch_info", "connect_info", "max_instance", "description", "project_name", "create_status", "server_id", "install_info", "account_required", "test_status", "online_status"}
	var (
		sb   strings.Builder
		args []interface{}
		idx  = 1
	)
	sb.WriteString("insert into ")
	sb.WriteString(m.defaultAeMcpExternalServicesConfigModel.table)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(columns, ","))
	sb.WriteString(") values ")
	for i, v := range list {
		if i > 0 {
			sb.WriteString(",")
		}
		// ($1,$2,...)
		sb.WriteString("(")
		for j := 0; j < len(columns); j++ {
			if j > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("$%d", idx))
			idx++
		}
		sb.WriteString(")")
		var launchArg interface{}
		if v.LaunchInfo == "" {
			launchArg = nil
		} else {
			launchArg = v.LaunchInfo
		}
		var connectArg interface{}
		if v.ConnectInfo == "" {
			connectArg = nil
		} else {
			connectArg = v.ConnectInfo
		}
		args = append(args, v.Name, v.Type, launchArg, connectArg, v.MaxInstance, v.Description, v.ProjectName, v.CreateStatus, v.ServerId, v.InstallInfo, v.AccountRequired, v.TestStatus, v.OnlineStatus)
	}
	_, err := m.defaultAeMcpExternalServicesConfigModel.conn.ExecCtx(ctx, sb.String(), args...)
	return err
}

func (m *customAeMcpExternalServicesConfigModel) GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpExternalServicesConfig, total int64, err error) {
	countQuery := fmt.Sprintf("select count(*) as number from %s", m.table)
	query := fmt.Sprintf("select %s from %s", aeMcpExternalServicesConfigRows, m.table)
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
		if len(lp.OrderBy) > 0 {
			query += " order by " + lp.OrderBy
		} else if len(lp.Sorts) > 0 && len(lp.Sorts[0].Filed) > 0 && len(lp.Sorts[0].Order) > 0 {
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

func (m *customAeMcpExternalServicesConfigModel) FindOneByCondition(ctx context.Context, conditions []models.Condition) (*AeMcpExternalServicesConfig, error) {
	query := fmt.Sprintf("select %s from %s", aeMcpExternalServicesConfigRows, m.table)
	//处理where条件
	whereClause, args, err := models.DealWithWhereSafe(conditions...)
	if err != nil {
		return nil, err
	}
	query += whereClause
	query += " limit 1"
	var resp AeMcpExternalServicesConfig
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

func (m *customAeMcpExternalServicesConfigModel) DeleteByConditions(ctx context.Context, conditions []models.Condition) error {
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
