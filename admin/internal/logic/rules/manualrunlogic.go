// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package rules

import (
	"context"
	"fmt"
	"strings"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ManualRunLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewManualRunLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ManualRunLogic {
	return &ManualRunLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ManualRunLogic) ManualRun(req *types.ManualRunReq) (resp *types.ManualRunResp, err error) {
	rule, err := l.svcCtx.RuleModel.FindOne(l.ctx, req.RuleId)
	if err != nil {
		l.Logger.Errorf("查询自动化规则失败: id=%d, err=%v", req.RuleId, err)
		return nil, err
	}
	operator := "system"
	//从上下文里取出“username”这个key，看是否有登录用户名
	if v := l.ctx.Value("username"); v != nil {
		if name := strings.TrimSpace(fmt.Sprintf("%v", v)); len(name) > 0 {
			operator = name //如果有值并且经过字符串转换，然后将操作人设置为当前上下文的name
		}
	} else if v := l.ctx.Value("userId"); v != nil {
		if uid := strings.TrimSpace(fmt.Sprintf("%v", v)); len(uid) > 0 {
			operator = uid //如果没有username，再尝试userid，退而求其次用userId作为操作者
		}
	}
	//真正执行规则的语句
	successCount, err := ExecuteRule(l.ctx, l.svcCtx, rule, "manual", operator)
	if err != nil {
		l.Logger.Errorf("执行自动化规则失败: ruleId=%d, err=%v", rule.Id, err)
		return nil, err
	}

	//返回给上层HTTP handler的响应结构体
	return &types.ManualRunResp{
		Success:   true,                                            //标记这次调用是成功的
		ExecCount: successCount,                                    //成功命中的用户数 / 写入执行日志的条数
		Message:   fmt.Sprintf("手动执行完成，写入执行日志 %d 条", successCount), //用户可读的提示信息
	}, nil
}
