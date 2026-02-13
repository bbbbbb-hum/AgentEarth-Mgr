package userfund

import (
	"context"
	"fmt"

	"AgentEarth-Mgr/admin/internal/svc"
)

// resolveOperatorName resolves operator based on charge source.
// - chargeSource == 1 or -1: use admin username
// - otherwise: use user username
func resolveOperatorName(ctx context.Context, svcCtx *svc.ServiceContext, userId string, chargeSource int64) string {
	if chargeSource == 1 || chargeSource == -1 {
		operatorId := ctx.Value("userId")
		if operatorId != nil {
			operatorIdStr := fmt.Sprintf("%v", operatorId)
			if len(operatorIdStr) > 0 {
				if user, err := svcCtx.UserModel.FindOneByUserId(ctx, operatorIdStr); err == nil && user != nil {
					if len(user.Username) > 0 {
						return user.Username
					}
				}
				return operatorIdStr
			}
		}
		return "unknown"
	}

	if len(userId) > 0 {
		if user, err := svcCtx.McpUserModel.FindOneByUserId(ctx, userId); err == nil && user != nil {
			if len(user.Username) > 0 {
				return user.Username
			}
		}
		return userId
	}

	return "unknown"
}

// resolveTargetUsername resolves username for the target user.
func resolveTargetUsername(ctx context.Context, svcCtx *svc.ServiceContext, userId string) string {
	if len(userId) > 0 {
		if user, err := svcCtx.McpUserModel.FindOneByUserId(ctx, userId); err == nil && user != nil {
			if len(user.Username) > 0 {
				return user.Username
			}
		}
		return userId
	}

	return "unknown"
}
