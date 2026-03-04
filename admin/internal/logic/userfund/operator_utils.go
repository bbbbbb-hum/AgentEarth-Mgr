package userfund

import (
	"context"
	"fmt"

	"AgentEarth-Mgr/admin/internal/svc"
)

// resolveOperatorName 根据场景解析“操作者名称”：
// - chargeSource 为 1 / 2 / -1 时：认为是后台/系统侧操作，优先用当前登录管理员用户名
//   （1：用户/产品逻辑单次充值，2：管理员单次操作，-1：系统扣减等）
// - 其他值：认为是用户侧行为，回退为目标用户的用户名
func resolveOperatorName(ctx context.Context, svcCtx *svc.ServiceContext, userId string, chargeSource int64) string {
	if chargeSource == 2 || chargeSource == -1 {
		// 优先使用上下文中的管理员用户名（由 JWT 中间件注入）
		if v := ctx.Value("username"); v != nil {
			if name := fmt.Sprintf("%v", v); len(name) > 0 && name != "<nil>" {
				return name
			}
		}
		// 其次使用管理员 userId 查 ae_mgrsystem_user 表
		operatorId := ctx.Value("userId")
		if operatorId != nil {
			operatorIdStr := fmt.Sprintf("%v", operatorId)
			if len(operatorIdStr) > 0 {
				if user, err := svcCtx.UserModel.FindOneByUserId(ctx, operatorIdStr); err == nil && user != nil {
					if len(user.Username) > 0 {
						return user.Username
					}
				}
				// 查不到用户名时，至少回退到 userId 本身
				return operatorIdStr
			}
		}
		// 无登录态时保留 "unknown"
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
