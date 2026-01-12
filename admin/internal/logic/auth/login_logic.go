package auth

import (
	"context"
	"errors"
	"time"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/models/users"

	"github.com/golang-jwt/jwt/v4"
	"github.com/zeromicro/go-zero/core/logx"
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
	user, err := l.svcCtx.UserModel.FindOneByUsername(l.ctx, req.Username)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return &types.LoginResp{
				Code:    401,
				Message: "用户名或密码错误",
			}, nil
		}
		return &types.LoginResp{
			Code:    500,
			Message: "服务器错误",
		}, err
	}

	if user.Status != "active" {
		return &types.LoginResp{
			Code:    403,
			Message: "账号已被禁用",
		}, nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return &types.LoginResp{
			Code:    401,
			Message: "用户名或密码错误",
		}, nil
	}

	token, err := l.generateToken(user.UserId, user.Username)
	if err != nil {
		return &types.LoginResp{
			Code:    500,
			Message: "生成token失败",
		}, err
	}

	user.LastLoginAt = time.Now()
	err = l.svcCtx.UserModel.Update(l.ctx, user)
	if err != nil {
		l.Errorf("更新最后登录时间失败: %v", err)
	}

	l.Infof("用户登录成功 - 用户ID: %s, 用户名: %s, 时间: %s", user.UserId, user.Username, time.Now().Format("2006-01-02 15:04:05"))

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

func (l *LoginLogic) generateToken(userId, username string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  userId,
		"username": username,
		"exp":      time.Now().Add(time.Duration(l.svcCtx.Config.Auth.AccessExpire) * time.Second).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(l.svcCtx.Config.Auth.AccessSecret))
}
