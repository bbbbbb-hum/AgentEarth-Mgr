package userfund

import (
	"context"
	"fmt"

	"AgentEarth-Mgr/admin/internal/svc"
)

// resolveOperatorName returns admin username if available; otherwise falls back to user.
func resolveOperatorName(ctx context.Context, svcCtx *svc.ServiceContext, userStrId string) string {
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

	if userStrId != "" {
		if user, err := svcCtx.McpUserModel.FindOneByUserStrId(ctx, userStrId); err == nil && user != nil {
			if user.Username != "" {
				return user.Username
			}
		}
	}

	return "unknown"
}
