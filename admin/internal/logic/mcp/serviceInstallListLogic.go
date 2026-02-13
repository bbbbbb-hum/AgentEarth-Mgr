package mcp

import (
	"AgentEarth-Mgr/models"
	"context"
	"strings"
	"unicode"

	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceInstallListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceInstallListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceInstallListLogic {
	return &ServiceInstallListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceInstallListLogic) ServiceInstallList(req *types.ServiceInstallListReq) (resp *types.BaseResp, err error) {
	// todo: add your logic here and delete this line
	var conditions []models.Condition
	if len(req.ServerId) > 0 {
		conditions = append(conditions, models.Condition{
			Field: "server_id",
			Value: req.ServerId,
		})
	}

	list, total, err := l.svcCtx.McpServicesInstallModel.GetList(l.ctx, models.ListConditions{
		Pages: models.Pages{
			Page: req.Page,
			Size: req.Size,
		},
		Conditions: conditions,
		Sorts: []models.Sort{
			{
				Filed: CamelToSnake(req.Sort),
				Order: req.Order,
			},
		},
	}, true)
	if err != nil {
		return
	}
	resp = &types.BaseResp{
		Code:    0,
		Message: "success",
		Data: types.D{
			"list":  list,
			"total": total,
		},
	}
	return
}

// CamelToSnake 将驼峰命名转换为蛇形命名
// 特别处理常见的缩写词，如 ID 转换为 id
func CamelToSnake(camel string) string {
	if len(camel) == 0 {
		return ""
	}

	// 预处理常见缩写
	camel = handleCommonAbbreviations(camel)

	var result strings.Builder
	for i, r := range camel {
		// 如果是大写字母
		if unicode.IsUpper(r) {
			// 不是在字符串开头且前一个字符不是下划线时添加下划线
			if i > 0 && camel[i-1] != '_' {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// handleCommonAbbreviations 处理常见的缩写词
func handleCommonAbbreviations(s string) string {
	// 处理 "ID" 相关的特殊情况
	s = strings.ReplaceAll(s, "ID", "Id")
	// 可以在这里添加更多常见的缩写处理
	// 例如: s = strings.ReplaceAll(s, "URL", "Url")
	//      s = strings.ReplaceAll(s, "HTTP", "Http")
	return s
}
