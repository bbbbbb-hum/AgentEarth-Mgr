// balance_helper 提供 logic 层统一的「实时余额」计算入口，SQL 在 models/userfund 层。
// 保留此文件便于：同一入口、后续扩展（日志/校验等）；调用方传入 UserFundModel（非事务用 svcCtx.UserFundModel，事务内用 WithSession(session)）。
package userfund

import (
	"context"

	"github.com/shopspring/decimal"
)

// FundModelForBalance 仅需实时余额计算，避免将 logic 与具体 model 包强绑定。
type FundModelForBalance interface {
	CalculateRealTimeBalance(ctx context.Context, recordID int64, initialAmount decimal.Decimal) (decimal.Decimal, error)
}

// CalculateRealTimeBalance 计算特定充值记录的实时余额。
// model 非事务时传 l.svcCtx.UserFundModel，事务内传 l.svcCtx.UserFundModel.WithSession(session)。
func CalculateRealTimeBalance(ctx context.Context, model FundModelForBalance, recordID int64, initialAmount decimal.Decimal) (decimal.Decimal, error) {
	return model.CalculateRealTimeBalance(ctx, recordID, initialAmount)
}
