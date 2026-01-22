// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth

import (
	"context"
	"errors"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/golang-jwt/jwt/v4"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"golang.org/x/crypto/bcrypt"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginReq) (resp *types.LoginResp, err error) {
	// 1. 查询用户
	user, err := l.svcCtx.UserModel.FindOneByUsername(l.ctx, req.Username)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return &types.LoginResp{
				Code:    401,
				Message: "用户不存在",
			}, nil
		}
		l.Errorf("查询用户失败: %v", err)
		return nil, err
	}

	// 2. 校验密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return &types.LoginResp{
			Code:    401,
			Message: "密码错误",
		}, nil
	}

	// 3. 生成 JWT Token
	now := time.Now().Unix()
	accessExpire := l.svcCtx.Config.Auth.AccessExpire
	token, err := l.getJwtToken(l.svcCtx.Config.Auth.AccessSecret, now, accessExpire, user.UserId)
	if err != nil {
		l.Errorf("生成Token失败: %v", err)
		return nil, err
	}

	// 4. 返回结果
	return &types.LoginResp{
		Code:    200,
		Message: "登录成功",
		Data: types.LoginData{
			Token:    token,
			UserId:   user.UserId,
			Username: user.Username,
		},
	}, nil
}

func (l *LoginLogic) getJwtToken(secretKey string, iat, seconds int64, userId string) (string, error) {
	claims := make(jwt.MapClaims)
	claims["exp"] = iat + seconds
	claims["iat"] = iat
	claims["userId"] = userId
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims
	return token.SignedString([]byte(secretKey))
}
