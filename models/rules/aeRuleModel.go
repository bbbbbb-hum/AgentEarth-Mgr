package rules

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AeRuleModel = (*customAeRuleModel)(nil)

type (
	// AeRuleModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeRuleModel.
	AeRuleModel interface {
		aeRuleModel
		withSession(session sqlx.Session) AeRuleModel
	}

	customAeRuleModel struct {
		*defaultAeRuleModel
	}
)

// NewAeRuleModel returns a model for the database table.
func NewAeRuleModel(conn sqlx.SqlConn) AeRuleModel {
	return &customAeRuleModel{
		defaultAeRuleModel: newAeRuleModel(conn),
	}
}

func (m *customAeRuleModel) withSession(session sqlx.Session) AeRuleModel {
	return NewAeRuleModel(sqlx.NewSqlConnFromSession(session))
}
