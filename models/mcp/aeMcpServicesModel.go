package mcp

import (
	"AgentEarth-Mgr/models"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeMcpServicesModel = (*customAeMcpServicesModel)(nil)

type (
	// AeMcpServicesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeMcpServicesModel.
	AeMcpServicesModel interface {
		aeMcpServicesModel
		withSession(session sqlx.Session) AeMcpServicesModel
		GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpServices, total int64, err error)
		GetAllTags(ctx context.Context) []string
		// InsertWithoutId inserts a service row without setting the auto-increment id column
		InsertWithoutId(ctx context.Context, data *AeMcpServices) (sql.Result, error)
		InsertWithId(ctx context.Context, data *AeMcpServices) (sql.Result, error)
		FindOneByCondition(ctx context.Context, conditions []models.Condition) (*AeMcpServices, error)
		// UpdateCreatedGroup 批量更新服务的启动分组
		UpdateCreatedGroup(ctx context.Context, ids []int64, createdGroup int64) error
		// BatchClose 批量关闭服务（将 enabled 更新为 false）
		BatchClose(ctx context.Context, ids []int64) error
		// BatchUpdatePrice 批量更新服务价格
		BatchUpdatePrice(ctx context.Context, ids []int64, price float64) (int64, error)
		// BatchUpdatePriceByCondition 根据条件批量更新服务价格
		BatchUpdatePriceByCondition(ctx context.Context, lp models.ListConditions, search string, price float64) (int64, error)
		// GetListWithSearch gets list with search query
		GetListWithSearch(ctx context.Context, lp models.ListConditions, search string, getList bool) (list []*AeMcpServices, total int64, err error)
	}

	customAeMcpServicesModel struct {
		*defaultAeMcpServicesModel
	}
)

// NewAeMcpServicesModel returns a model for the database table.
func NewAeMcpServicesModel(conn sqlx.SqlConn) AeMcpServicesModel {
	return &customAeMcpServicesModel{
		defaultAeMcpServicesModel: newAeMcpServicesModel(conn),
	}
}

func (m *customAeMcpServicesModel) withSession(session sqlx.Session) AeMcpServicesModel {
	return NewAeMcpServicesModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAeMcpServicesModel) GetListWithSearch(ctx context.Context, lp models.ListConditions, search string, getList bool) (list []*AeMcpServices, total int64, err error) {
	countQuery := fmt.Sprintf("select count(*) as number from %s", m.table)
	query := fmt.Sprintf("select %s from %s", aeMcpServicesRows, m.table)

	// Process standard conditions
	whereClause, args, err := models.DealWithWhereSafe(lp.Conditions...)
	if err != nil {
		return
	}

	// Handle search (ID or Name)
	if len(search) > 0 {
		prefix := " WHERE"
		if len(whereClause) > 0 {
			prefix = " AND"
		}

		argIdx := len(args) + 1
		nameSearch := "%" + search + "%"

		if id, err := strconv.ParseInt(search, 10, 64); err == nil {
			// Search by ID OR Name
			whereClause += fmt.Sprintf("%s (id = $%d OR server_name ILIKE $%d OR server_id ILIKE $%d)", prefix, argIdx, argIdx+1, argIdx+2)
			args = append(args, id, nameSearch, nameSearch)
		} else {
			// Search by Name only
			whereClause += fmt.Sprintf("%s (server_name ILIKE $%d OR server_id ILIKE $%d)", prefix, argIdx, argIdx+1)
			args = append(args, nameSearch, nameSearch)
		}
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
			// 如果是搜索且没有手动排序，则 STRPOS 优先
			if len(search) > 0 && len(lp.Sorts) >= 2 && lp.Sorts[0].Filed == "enabled" {
				argIdx := len(args) + 1
				// 直接构建排序，不调用 GetOrderBy，避免 double order by 或 split 失败
				query += fmt.Sprintf(" ORDER BY STRPOS(LOWER(server_name), LOWER($%d)) ASC, enabled DESC, id DESC", argIdx)
				args = append(args, search)
			} else {
				query = models.GetOrderBy(lp.Sorts, query)
			}
		} else {
			if len(search) > 0 {
				argIdx := len(args) + 1
				query += fmt.Sprintf(" ORDER BY STRPOS(LOWER(server_name), LOWER($%d)) ASC, enabled DESC, id DESC", argIdx)
				args = append(args, search)
			} else {
				query += " ORDER BY enabled DESC, id DESC"
			}
		}
		if lp.Page > 0 && lp.Size > 0 {
			var offset = (lp.Page - 1) * lp.Size
			query += fmt.Sprintf(" limit %d offset %d", lp.Size, offset)
		}
		err = m.conn.QueryRowsCtx(ctx, &list, query, args...)
	}
	return
}

func (m *customAeMcpServicesModel) GetList(ctx context.Context, lp models.ListConditions, getList bool) (list []*AeMcpServices, total int64, err error) {
	countQuery := fmt.Sprintf("select count(*) as number from %s", m.table)
	query := fmt.Sprintf("select %s from %s", aeMcpServicesRows, m.table)
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
			query += " ORDER BY enabled DESC, id DESC"
		}
		if lp.Page > 0 && lp.Size > 0 {
			var offset = (lp.Page - 1) * lp.Size
			query += fmt.Sprintf(" limit %d offset %d", lp.Size, offset)
		}
		err = m.conn.QueryRowsCtx(ctx, &list, query, args...)
	}
	return
}

func (m *customAeMcpServicesModel) GetAllTags(ctx context.Context) []string {
	var tags []string
	query := fmt.Sprintf("SELECT DISTINCT unnest(tags) as tag FROM %s WHERE tags IS NOT NULL AND array_length(tags, 1) > 0 ORDER BY tag", m.table)
	err := m.conn.QueryRowsCtx(ctx, &tags, query)
	if err != nil {
		return []string{}
	}
	return tags
}

// InsertWithoutId inserts without providing the auto-increment id.
func (m *customAeMcpServicesModel) InsertWithoutId(ctx context.Context, data *AeMcpServices) (sql.Result, error) {
	// Explicit column list without id
	columns := "server_id, server_name, logo, protocol_version, enabled, tags, description, task_chain_id, x_net_service_id, call_num,project_name,is_install,xlcredit_price"
	query := fmt.Sprintf("insert into %s (%s) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)", m.table, columns)
	return m.conn.ExecCtx(ctx, query, data.ServerId, data.ServerName, data.Logo, data.ProtocolVersion, data.Enabled, data.Tags, data.Description, data.TaskChainId, data.XNetServiceId, data.CallNum, data.ProjectName, data.IsInstall, data.XlcreditPrice)
}

func (m *customAeMcpServicesModel) InsertWithId(ctx context.Context, data *AeMcpServices) (sql.Result, error) {
	columns := "id, server_id, server_name, logo, protocol_version, enabled, tags, description, task_chain_id, x_net_service_id, call_num,project_name,is_install,xlcredit_price"
	query := fmt.Sprintf("insert into %s (%s) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)", m.table, columns)
	return m.conn.ExecCtx(ctx, query, data.Id, data.ServerId, data.ServerName, data.Logo, data.ProtocolVersion, data.Enabled, data.Tags, data.Description, data.TaskChainId, data.XNetServiceId, data.CallNum, data.ProjectName, data.IsInstall, data.XlcreditPrice)
}

func (m *customAeMcpServicesModel) FindOneByCondition(ctx context.Context, conditions []models.Condition) (*AeMcpServices, error) {
	query := fmt.Sprintf("select %s from %s", aeMcpServicesRows, m.table)
	//处理where条件
	whereClause, args, err1 := models.DealWithWhereSafe(conditions...)
	if err1 != nil {
		return nil, err1
	}
	query += whereClause
	query += " limit 1"
	var resp AeMcpServices
	err := m.conn.QueryRowCtx(ctx, &resp, query, args...)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// UpdateCreatedGroup 批量更新服务的启动分组
func (m *customAeMcpServicesModel) UpdateCreatedGroup(ctx context.Context, ids []int64, createdGroup int64) error {
	if len(ids) == 0 {
		return nil
	}
	// 构建 IN 子句的占位符
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = createdGroup
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}
	query := fmt.Sprintf("update %s set created_group = $1 where id in (%s)", m.table, strings.Join(placeholders, ","))
	_, err := m.conn.ExecCtx(ctx, query, args...)
	return err
}

// BatchClose 批量关闭服务（将 enabled 更新为 false）
func (m *customAeMcpServicesModel) BatchClose(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	// 构建 IN 子句的占位符
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf("update %s set enabled = false where id in (%s)", m.table, strings.Join(placeholders, ","))
	_, err := m.conn.ExecCtx(ctx, query, args...)
	return err
}

// BatchUpdatePrice 批量更新服务价格
func (m *customAeMcpServicesModel) BatchUpdatePrice(ctx context.Context, ids []int64, price float64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// 构建 IN 子句的占位符
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = price
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}
	query := fmt.Sprintf("update %s set xlcredit_price = $1 where id in (%s)", m.table, strings.Join(placeholders, ","))
	result, err := m.conn.ExecCtx(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// BatchUpdatePriceByCondition 根据条件批量更新服务价格
func (m *customAeMcpServicesModel) BatchUpdatePriceByCondition(ctx context.Context, lp models.ListConditions, search string, price float64) (int64, error) {
	// Process standard conditions
	whereClause, args, err := models.DealWithWhereSafe(lp.Conditions...)
	if err != nil {
		return 0, err
	}

	// Handle search (ID or Name)
	if len(search) > 0 {
		prefix := " WHERE"
		if len(whereClause) > 0 {
			prefix = " AND"
		}

		argIdx := len(args) + 1
		nameSearch := "%" + search + "%"

		if id, err := strconv.ParseInt(search, 10, 64); err == nil {
			// Search by ID OR Name
			whereClause += fmt.Sprintf("%s (id = $%d OR server_name ILIKE $%d)", prefix, argIdx, argIdx+1)
			args = append(args, id, nameSearch)
		} else {
			// Search by Name only
			whereClause += fmt.Sprintf("%s (server_name ILIKE $%d)", prefix, argIdx)
			args = append(args, nameSearch)
		}
	}

	// Insert price as the first argument for the update statement
	// Current args are for the WHERE clause
	// We need to shift all $n in whereClause by 1, and prepend price to args
	// BUT, manual string manipulation of $n is error prone.
	// EASIER: Use named args? Go sql doesn't support named args standardly.
	// ALTERNATIVE: Rebuild the query carefully.

	// Let's restart the arg construction.
	// updateArgs := []interface{}{price} // $1 is price

	// Re-process standard conditions with offset 1 (because $1 is price)
	// Wait, DealWithWhereSafe starts with $1.
	// We can let DealWithWhereSafe generate $1..$N, and we prepend "update ... set price=$1 where ..."
	// No, price is $1, so where clause must start with $2.

	// Since we can't easily change the start index of DealWithWhereSafe (assuming it's a fixed helper),
	// We might need to manually adjust the whereClause string or use a different approach.
	// If DealWithWhereSafe returns "WHERE field = $1", we can replace "$1" with "$2", "$2" with "$3"...
	// This is risky.

	// BETTER: Append price to the END of args?
	// UPDATE table SET price = $N+1 WHERE ...
	// Yes.

	finalArgs := args
	finalArgs = append(finalArgs, price)
	priceArgIndex := len(finalArgs)

	query := fmt.Sprintf("update %s set xlcredit_price = $%d %s", m.table, priceArgIndex, whereClause)

	result, err := m.conn.ExecCtx(ctx, query, finalArgs...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
