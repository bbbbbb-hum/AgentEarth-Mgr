package rules

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AeRuleExecutionLogModel = (*customAeRuleExecutionLogModel)(nil)

type (
	// AeRuleExecutionLogModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAeRuleExecutionLogModel.
	AeRuleExecutionLogModel interface {
		aeRuleExecutionLogModel
		withSession(session sqlx.Session) AeRuleExecutionLogModel
	}

	customAeRuleExecutionLogModel struct {
		*defaultAeRuleExecutionLogModel
	}
)

// NewAeRuleExecutionLogModel returns a model for the database table.
func NewAeRuleExecutionLogModel(conn sqlx.SqlConn) AeRuleExecutionLogModel {
	return &customAeRuleExecutionLogModel{
		defaultAeRuleExecutionLogModel: newAeRuleExecutionLogModel(conn),
	}
}

func (m *customAeRuleExecutionLogModel) withSession(session sqlx.Session) AeRuleExecutionLogModel {
	return NewAeRuleExecutionLogModel(sqlx.NewSqlConnFromSession(session))
}
