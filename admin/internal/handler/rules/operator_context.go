package rules

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"AgentEarth-Mgr/admin/internal/svc"

	"github.com/golang-jwt/jwt/v4"
)

// withOperatorIdentityCtx 从 Authorization 中提取管理员信息，写入上下文。
func withOperatorIdentityCtx(r *http.Request, svcCtx *svc.ServiceContext) context.Context {
	ctx := r.Context()
	auth := r.Header.Get("Authorization")
	if len(auth) == 0 {
		return ctx
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ctx
	}
	tokenStr := strings.TrimSpace(parts[1])
	if len(tokenStr) == 0 {
		return ctx
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(svcCtx.Config.Auth.AccessSecret), nil
	})
	if err != nil || token == nil || !token.Valid {
		return ctx
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ctx
	}

	userID := strings.TrimSpace(fmt.Sprintf("%v", claims["userId"]))
	username := strings.TrimSpace(fmt.Sprintf("%v", claims["username"]))
	if (len(username) == 0 || username == "<nil>") && len(userID) > 0 {
		if user, qErr := svcCtx.UserModel.FindOneByUserId(ctx, userID); qErr == nil && user != nil {
			username = strings.TrimSpace(user.Username)
		}
	}

	newCtx := ctx
	if len(userID) > 0 {
		newCtx = context.WithValue(newCtx, "userId", userID)
	}
	if len(username) > 0 && username != "<nil>" {
		newCtx = context.WithValue(newCtx, "username", username)
	}
	return newCtx
}
