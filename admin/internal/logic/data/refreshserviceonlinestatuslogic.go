package data

import (
	"AgentEarth-Mgr/admin/internal/svc"
	"AgentEarth-Mgr/admin/internal/types"
	"AgentEarth-Mgr/models"
	"context"
	"os"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type RefreshServiceOnlineStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshServiceOnlineStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshServiceOnlineStatusLogic {
	return &RefreshServiceOnlineStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefreshServiceOnlineStatusLogic) RefreshServiceOnlineStatus(req *types.GetServiceConfigListReq) (resp *types.BaseResp, err error) {
	syncMessage := ""

	if l.svcCtx.Config.K8sSync.Enabled {
		readyNames, syncErr := l.syncFromK8s()
		if syncErr != nil {
			l.Logger.Errorf("K8s 状态同步失败: %v", syncErr)
			syncMessage = "K8s 状态同步失败: " + syncErr.Error()
		} else {
			if updateErr := l.svcCtx.TaskNodeConfigV2Model.BatchUpdateOnlineStatus(l.ctx, readyNames); updateErr != nil {
				l.Logger.Errorf("批量更新 online_status 失败: %v", updateErr)
				syncMessage = "更新状态失败: " + updateErr.Error()
			} else {
				syncMessage = "已同步集群状态"
			}
		}
	} else {
		syncMessage = "K8s 同步未启用，跳过状态检测"
	}

	// 复用 GetServiceConfigList 逻辑查询列表
	listLogic := NewGetServiceConfigListLogic(l.ctx, l.svcCtx)
	listResp, listErr := listLogic.GetServiceConfigList(req)
	if listErr != nil {
		return nil, listErr
	}

	// 在返回的 data 中附加 sync_message
	if listResp != nil && listResp.Data != nil {
		listResp.Data["sync_message"] = syncMessage
	}
	return listResp, nil
}

// syncFromK8s 连接 K8s API，拉取 Running+Ready 的 wemcp Pod，返回就绪的 wemcp_name 列表。
func (l *RefreshServiceOnlineStatusLogic) syncFromK8s() ([]string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	namespace := l.svcCtx.Config.K8sSync.Namespace
	if len(namespace) == 0 {
		nsBytes, readErr := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if readErr != nil {
			return nil, readErr
		}
		namespace = strings.TrimSpace(string(nsBytes))
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(l.ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=backend",
	})
	if err != nil {
		return nil, err
	}

	// 从 DB 获取所有已录入的 wemcp_name，用于过滤
	allConfigs, _, dbErr := l.svcCtx.TaskNodeConfigV2Model.GetList(l.ctx, models.ListConditions{
		Pages: models.Pages{Page: 1, Size: 10000},
	}, true)
	if dbErr != nil {
		return nil, dbErr
	}
	knownNames := make(map[string]struct{}, len(allConfigs))
	for _, cfg := range allConfigs {
		knownNames[cfg.WemcpName] = struct{}{}
	}

	readySet := make(map[string]struct{})
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if !isPodReady(&pod) {
			continue
		}

		// 从 label "app" 提取实例名（如 ae-wemcp2-qweather）
		appLabel := pod.Labels["app"]
		if len(appLabel) == 0 {
			appLabel = pod.Labels["app.kubernetes.io/instance"]
		}
		if len(appLabel) == 0 {
			continue
		}

		// 去掉 ae- 前缀得到 wemcp_name（如 wemcp2-qweather）
		wemcpName := strings.TrimPrefix(appLabel, "ae-")

		// 仅匹配已录入的服务
		if _, ok := knownNames[wemcpName]; ok {
			readySet[wemcpName] = struct{}{}
		}
	}

	result := make([]string, 0, len(readySet))
	for name := range readySet {
		result = append(result, name)
	}
	return result, nil
}

// isPodReady 检查 Pod 的 Ready condition 是否为 True。
func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
