package userfund

import (
	"context"
	"fmt"

	"AgentEarth-Mgr/admin/internal/svc"
)

// resolveOperatorName 根据场景解析“操作者名称”：
// 这里的 chargeSource 语义与 ae_user_recharge_record.charge_source 一致：
// 1：用户自主操作；
// 2：运营手工单个操作；
// 3：运营手工批量操作；
// 4：自动 Rule 操作；
// 5：非 cron_rule 的业务逻辑（过期扣减）；
// -1：其他。
//
// 规则：
// - chargeSource == 1 时，认为是“用户自主操作”，operator 回退为目标用户的用户名；
// - 其他情况认为是后台/系统侧操作，优先使用当前登录管理员用户名。
func resolveOperatorName(ctx context.Context, svcCtx *svc.ServiceContext, userId string, chargeSource int64) string {
	// 用户自主操作：直接用目标用户信息
	if chargeSource == 1 {
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

	// 非 1 的场景视为后台/系统侧操作：优先取当前登录管理员用户名
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
