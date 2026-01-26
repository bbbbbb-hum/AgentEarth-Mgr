package userfund

import (
	"context"
	"fmt"

	"AgentEarth-Mgr/admin/internal/svc"
)

// resolveOperatorName resolves operator based on charge source.
// - chargeSource == 1 or -1: use admin username
// - otherwise: use user username
func resolveOperatorName(ctx context.Context, svcCtx *svc.ServiceContext, userStrId string, chargeSource int64) string {
	if chargeSource == 1 || chargeSource == -1 {
		operatorId := ctx.Value("userId")
		if operatorId != nil {
			operatorIdStr := fmt.Sprintf("%v", operatorId)
			if operatorIdStr != "" {
				if user, err := svcCtx.UserModel.FindOneByUserId(ctx, operatorIdStr); err == nil && user != nil {
					if user.Username != "" {
						return user.Username
					}
				}
				return operatorIdStr
			}
		}
		return "unknown"
	}

	if userStrId != "" {
		if user, err := svcCtx.McpUserModel.FindOneByUserStrId(ctx, userStrId); err == nil && user != nil {
			if user.Username != "" {
				return user.Username
			}
		}
		return userStrId
	}

	return "unknown"
}
