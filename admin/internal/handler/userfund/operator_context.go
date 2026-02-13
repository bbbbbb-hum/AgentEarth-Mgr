package userfund

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"AgentEarth-Mgr/admin/internal/svc"

	"github.com/golang-jwt/jwt/v4"
)

func withOperatorUsernameCtx(r *http.Request, svcCtx *svc.ServiceContext) context.Context {
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

	userIdVal, hasUserId := claims["userId"]
	usernameVal, ok := claims["username"]
	var username string
	if ok {
		username = fmt.Sprintf("%v", usernameVal)
	}

	if len(username) == 0 && hasUserId {
		userId := fmt.Sprintf("%v", userIdVal)
		if len(userId) > 0 {
			var result struct {
				Username string `db:"username"`
			}
			query := fmt.Sprintf(`select username from %s where user_id = $1 limit 1`, svcCtx.UserModel.TableName())
			if err := svcCtx.DB.QueryRowCtx(ctx, &result, query, userId); err == nil {
				if len(result.Username) > 0 {
					username = result.Username
				}
			}
		}
	}

	if len(username) == 0 && !hasUserId {
		return ctx
	}

	newCtx := ctx
	if hasUserId {
		userId := fmt.Sprintf("%v", userIdVal)
		if len(userId) > 0 {
			newCtx = context.WithValue(newCtx, "userId", userId)
		}
	}
	if len(username) > 0 {
		newCtx = context.WithValue(newCtx, "username", username)
	}

	return newCtx
}
