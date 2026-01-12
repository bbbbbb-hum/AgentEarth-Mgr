package users

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AeMcpExternalServicesUserModel = (*customAeMcpExternalServicesUserModel)(nil)

type (
	// AeMcpExternalServicesUserModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeMcpExternalServicesUserModel.
	AeMcpExternalServicesUserModel interface {
		aeMcpExternalServicesUserModel
		withSession(session sqlx.Session) AeMcpExternalServicesUserModel
		FindOneByUsername(ctx context.Context, username string) (*AeMcpExternalServicesUser, error)
	}

	customAeMcpExternalServicesUserModel struct {
		*defaultAeMcpExternalServicesUserModel
	}
)

// NewAeMcpExternalServicesUserModel returns a model for the database table.
func NewAeMcpExternalServicesUserModel(conn sqlx.SqlConn) AeMcpExternalServicesUserModel {
	return &customAeMcpExternalServicesUserModel{
		defaultAeMcpExternalServicesUserModel: newAeMcpExternalServicesUserModel(conn),
	}
}

func (m *customAeMcpExternalServicesUserModel) withSession(session sqlx.Session) AeMcpExternalServicesUserModel {
	return NewAeMcpExternalServicesUserModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAeMcpExternalServicesUserModel) FindOneByUsername(ctx context.Context, username string) (*AeMcpExternalServicesUser, error) {
	var resp AeMcpExternalServicesUser
	query := fmt.Sprintf("select %s from %s where username = $1 limit 1", aeMcpExternalServicesUserRows, m.table)
	err := m.conn.QueryRowCtx(ctx, &resp, query, username)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
