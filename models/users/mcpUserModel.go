package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ McpUserModel = (*customMcpUserModel)(nil)

type (
	// McpUserModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMcpUserModel.
	McpUserModel interface {
		mcpUserModel
		withSession(session sqlx.Session) McpUserModel
	}

	customMcpUserModel struct {
		*defaultMcpUserModel
	}
)

// NewMcpUserModel returns a model for the database table.
func NewMcpUserModel(conn sqlx.SqlConn) McpUserModel {
	return &customMcpUserModel{
		defaultMcpUserModel: newMcpUserModel(conn),
	}
}

func (m *customMcpUserModel) withSession(session sqlx.Session) McpUserModel {
	return NewMcpUserModel(sqlx.NewSqlConnFromSession(session))
}

// FindList 重写了生成文件中的方法，添加了更智能的搜索相关度排序
func (m *customMcpUserModel) FindList(ctx context.Context, page, pageSize int, search, status string) ([]*McpUser, int64, error) {
	var users []*McpUser
	var count int64

	whereConditions := []string{}
	var (
		argsForCount []interface{}
		argsForList  []interface{}
	)
	
	paramIndex := 1

	// 搜索逻辑：
	// - 同时在 user_id / phone / username / email 字段上模糊匹配
	// - 忽略大小写（使用 ILIKE）
	// - 排序按匹配位置优先：前缀匹配（position = 1）最前，其次位置越靠前越优先
	if len(search) > 0 {
		// 使用 ILIKE 做不区分大小写的模糊匹配
		pattern := "%" + search + "%"
		keywordLower := strings.ToLower(search)

		searchCondition := fmt.Sprintf(`(
			username ILIKE $%d OR
			phone ILIKE $%d OR
			email ILIKE $%d OR
			user_id::text ILIKE $%d
		)`, paramIndex, paramIndex, paramIndex, paramIndex)
		
		whereConditions = append(whereConditions, searchCondition)
		argsForCount = append(argsForCount, pattern)
		argsForList = append(argsForList, pattern)
		paramIndex++
		
		// list 查询需要 keywordLower 用于 position() 计算匹配位置
		argsForList = append(argsForList, keywordLower)
	}
	
	// 状态筛选逻辑：
	// - active: 最近30天内登录过
	// - inactive: 超过30天未登录或从未登录
	if status == "active" {
		whereConditions = append(whereConditions, "last_login_at >= CURRENT_DATE - INTERVAL '30 days'")
	} else if status == "inactive" {
		whereConditions = append(whereConditions, "(last_login_at < CURRENT_DATE - INTERVAL '30 days' OR last_login_at IS NULL)")
	}
	
	where := "1=1"
	if len(whereConditions) > 0 {
		where = strings.Join(whereConditions, " AND ")
	}

	countQuery := fmt.Sprintf("select count(*) from %s where %s", m.table, where)
	err := m.conn.QueryRowCtx(ctx, &count, countQuery, argsForCount...)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var query string

	if len(search) > 0 {
		// 带搜索时：先按匹配位置排序，再按最近登录时间
		// position() 在 PostgreSQL 中是从 1 开始；我们用 LEAST + NULLIF(…,0) 取四个字段中最靠前的位置
		query = fmt.Sprintf(`
			select %s
			from %s
			where %s
			order by
				LEAST(
					NULLIF(position($2 in LOWER(username)), 0),
					NULLIF(position($2 in LOWER(phone)), 0),
					NULLIF(position($2 in LOWER(email)), 0),
					NULLIF(position($2 in LOWER(user_id::text)), 0)
				) asc,
				last_login_at desc nulls last
			limit $3 offset $4
		`, mcpUserRows, m.table, where)

		argsForList = append(argsForList, pageSize, offset)
		err = m.conn.QueryRowsCtx(ctx, &users, query, argsForList...)
	} else {
		// 未搜索时：保持原来的按最近活跃排序
		query = fmt.Sprintf(`
			select %s
			from %s
			where %s
			order by last_login_at desc nulls last
			limit $1 offset $2
		`, mcpUserRows, m.table, where)

		err = m.conn.QueryRowsCtx(ctx, &users, query, pageSize, offset)
	}

	return users, count, err
}
