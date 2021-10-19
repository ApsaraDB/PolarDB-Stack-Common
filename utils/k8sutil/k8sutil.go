/* 
*Copyright (c) 2019-2021, Alibaba Group Holding Limited;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
 */


package k8sutil

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ApsaraDB/PolarDB-Stack-Common/define"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/retryutil"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
)

type NodeClientIP struct {
	NodeName string
	ClientIP string
	HostIP   string
}

/**
 * @Description: 获得所有节点的客户网络IP
 * @return []*NodeClientIP
 * @return error
 */
func GetNodeClientIPList(logger logr.Logger) ([]*NodeClientIP, error) {
	var nodesClientIP []*NodeClientIP
	cacheProvider := utils.GetCacheProvider()
	ips, exist, err := cacheProvider.Get(define.CacheKeyNodesClientIP)
	if err == nil && exist {
		nodesClientIP = ips.([]*NodeClientIP)
		if len(nodesClientIP) > 0 {
			return nodesClientIP, nil
		}
	}
	nodes, err := GetNodes(logger)
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		nodeClientIP := &NodeClientIP{NodeName: node.Name}
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				nodeClientIP.HostIP = address.Address
			}
		}
		for _, condition := range node.Status.Conditions {
			if condition.Type == "NodeClientIP" &&
				condition.Status == corev1.ConditionTrue &&
				condition.Message != "0.0.0.0" {
				nodeClientIP.ClientIP = condition.Message
			}
		}
		nodesClientIP = append(nodesClientIP, nodeClientIP)
	}
	if len(nodesClientIP) > 0 {
		cacheProvider.Add(define.CacheKeyNodesClientIP, nodesClientIP)
	}
	return nodesClientIP, nil
}

func createClientIPNotFoundError(ip string) error {
	return errors.New(fmt.Sprintf("[%s] client ip not found.", ip))
}

func createHostIPNotFoundError(ip string) error {
	return errors.New(fmt.Sprintf("[%s] host ip not found.", ip))
}

/**
 * @Description: 根据主机IP获得客户网络IP地址
 * @param hostIP
 * @param nodeClientIPList
 * @return string
 */
func GetClientIP(hostIP string, logger logr.Logger) (string, error) {
	nodeClientIPList, err := GetNodeClientIPList(logger)
	if err != nil {
		return "", err
	}
	for _, clientIP := range nodeClientIPList {
		if clientIP.HostIP == hostIP && clientIP.ClientIP != "" {
			return clientIP.ClientIP, nil
		}
	}
	return "", createClientIPNotFoundError(hostIP)
}

/**
 * @Description: 根据ClientIP获取HostIP
 * @param hostIP
 * @param nodeClientIPList
 * @return string
 */
func GetHostIP(clientIP string, logger logr.Logger) (string, error) {
	nodeClientIPList, err := GetNodeClientIPList(logger)
	if err != nil {
		return "", err
	}
	for _, ip := range nodeClientIPList {
		if ip.ClientIP == clientIP && ip.HostIP != "" {
			return ip.HostIP, nil
		}
	}
	return "", createHostIPNotFoundError(clientIP)
}

func GetService(name string, namespace string, logger logr.Logger) (*corev1.Service, error) {
	service := &v1.Service{}
	if err := mgr.GetSyncClient().Get(context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace}, service); err != nil {
		logger.Error(err, "GetService failed")
		return nil, err
	}
	return service, nil
}

func GetServiceByLabel(labelStr string, namespace string, logger logr.Logger) (*corev1.ServiceList, error) {
	labelSelector, err := labels.Parse(labelStr)
	if err != nil {
		return nil, err
	}
	serviceList := &v1.ServiceList{}
	if err := mgr.GetSyncClient().List(context.TODO(), serviceList, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	}); err != nil {
		return nil, err
	}
	return serviceList, nil
}

func GetConfigMap(name, namespace string, logger logr.Logger) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	if err := mgr.GetSyncClient().Get(context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace}, configMap); err != nil {
		logger.Error(err, "GetConfigMap error", "name", name, "namespace", namespace)
		return nil, err
	}
	return configMap, nil
}

func UpdateConfigMap(configMap *corev1.ConfigMap) error {
	return mgr.GetSyncClient().Update(context.TODO(), configMap)
}

func DeleteConfigmap(cm *corev1.ConfigMap, logger logr.Logger) error {
	if err := mgr.GetSyncClient().Delete(context.TODO(), cm); err != nil {
		logger.Error(err, "delete configmap failed", "name", cm.Name, "namespace", cm.Namespace)
		return err
	}
	return nil
}

func CreatePod(pod *v1.Pod, logger logr.Logger) error {
	client, err := GetCorev1Client()
	if err != nil {
		logger.Error(err, "GetCorev1Client failed")
		return err
	}
	pod, err = client.Pods(pod.Namespace).Create(pod)
	return err
}

func GetPod(name, namespace string, logger logr.Logger) (*v1.Pod, error) {
	client, err := GetCorev1Client()
	if err != nil {
		logger.Error(err, "GetCorev1Client failed")
		return nil, err
	}
	return client.Pods(namespace).Get(name, metav1.GetOptions{})
}

func GetPodsByLabel(labelStr, namespace string, logger logr.Logger) (*v1.PodList, error) {
	client, err := GetCorev1Client()
	if err != nil {
		logger.Error(err, "GetCorev1Client failed")
		return nil, err
	}
	return client.Pods(namespace).List(metav1.ListOptions{
		LabelSelector: labelStr,
	})
}

func DeletePod(name, namespace string, ctx context.Context, logger logr.Logger) error {
	client, err := GetCorev1Client()
	if err != nil {
		logger.Error(err, "GetCorev1Client failed")
		return err
	}
	if err := client.Pods(namespace).Delete(name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := WaitForPodDeleted(namespace, name, ctx, logger); err != nil {
		err = errors.Wrapf(err, "wait pod %s deleted error", name)
		logger.Error(err, "")
		return err
	}
	return nil
}

func CheckPodReady(name, namespace string, logger logr.Logger) bool {
	client, err := GetCorev1Client()
	if err != nil {
		logger.Error(err, "GetCorev1Client failed")
		return false
	}
	newPod, getErr := client.Pods(namespace).Get(name, metav1.GetOptions{})
	if getErr != nil {
		logger.Error(err, "GetPod failed")
		return false
	}
	for _, cond := range newPod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func WaitForPodReady(pod *corev1.Pod, isNewPod bool, ctx context.Context, logger logr.Logger) (*corev1.Pod, error) {
	coreClient, err := GetCorev1Client()
	if err != nil {
		logger.Error(err, "GetCorev1Client failed")
		return nil, errors.Wrap(err, "get core v1 client error ")
	}
	var innerErr string
	err = retryutil.RetryFunc(ctx, time.Second, 3, func() (bool, error) {
		// Wait for the Pod to indicate Ready == True.
		podOpt := SetWatchOption(pod.ObjectMeta, 900)
		// 设置当前watch的resource 从头开始，确认pod曾经ready过.
		if isNewPod {
			podOpt.ResourceVersion = ""
		} else {
			if rv := GetPodWatchResourceVersion(pod.Name, pod.Namespace, coreClient, logger); rv != "" {
				podOpt.ResourceVersion = rv
			}
		}
		watcher, err := coreClient.Pods(pod.Namespace).Watch(*podOpt)
		defer func() {
			if watcher != nil {
				watcher.Stop()
			}
		}()
		if err != nil {
			logger.Error(err, "watch pod error", "name", pod.Name)
			return true, errors.Wrap(err, "watch pod error")
		}
		for {
			select {
			case event := <-watcher.ResultChan():
				switch event.Type {
				case watch.Error:
					// watch到error重试，
					innerErr = fmt.Sprintf("[%s]event type is ERROR: %s when wait for pod ready.[%v]", pod.Name, event.Type, event)
					logger.Error(errors.New(innerErr), "")
					return false, errors.New(innerErr)
				// 如果是 watch.Added event type, 继续执行下一个case, 并且不会判断case 表达式.
				case watch.Added:
					fallthrough
				case watch.Modified:
					pod = event.Object.(*corev1.Pod)
					for _, cond := range pod.Status.Conditions {
						logger.Info(fmt.Sprintf("%v get an event, %v status: %v", pod.Name, cond.Type, cond.Status))
						if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
							goto QUIT
						}

						if IsPodSchedulerFailed(&cond) {
							logger.Error(errors.New(fmt.Sprintf("[%s] found scheduler failed, msg: %s", pod.Name, StringOfPodCondition(&cond))), "")
							goto QUIT
						}
					}
					continue
				default:
					pName := ""
					if pod != nil {
						pName = pod.Name
					}
					logger.Error(errors.New(fmt.Sprintf("[%s]unexpected event type [%s] when wait for pod ready.[%v]", pName, event.Type, event)), "")
					return true, errors.Errorf("unexpected event type %s when wait for pod ready, error: %v", string(event.Type), event)
				}

			case <-ctx.Done():
				err = errors.New("cancelled when wait for engine pod ready.")
				logger.Error(err, "")
				return true, err
			}
		QUIT:
			break
		}
		return true, nil
	})

	if innerErr != "" && err != nil {
		err = errors.Wrap(err, innerErr)
	}

	if err != nil {
		return pod, err
	}

	time.Sleep(1 * time.Second)

	logger.Info("try to get pod latest status.", "name", pod.Name)
	newPod, getErr := coreClient.Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
	if getErr != nil {
		logger.Error(err, "get pod error", "name", pod.Name)
		if apierrors.IsNotFound(getErr) {
			return pod, define.CreateInterruptError(define.InterruptPodLost, getErr)
		}
		return pod, getErr
	}

	var containerReady = false
	var podReady = false

	// 确认已经ready的pod, 当前状态, 如果pod启动失败直接报错退出.
	for _, cond := range newPod.Status.Conditions {
		if cond.Type == corev1.ContainersReady && cond.Status == corev1.ConditionTrue {
			containerReady = true
			logger.Info("pod container has go to ready", "name", pod.Name)
		}

		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			podReady = true
			logger.Info("pod has go to ready", "name", pod.Name)
		}

		if IsPodSchedulerFailed(&cond) {
			err := errors.New(fmt.Sprintf("pod [%s] schedule failed, may caused by insufficient resources. trace: %s", pod.Name, cond.Message))
			logger.Error(err, "")
			return pod, define.CreateInterruptError(define.InterruptPodUnScheduled, err)
		}

		if cond.Type == corev1.ContainersReady && cond.Status == corev1.ConditionFalse {
			podStatusInfo := GetPodContainerInfo(newPod, logger)
			logger.Error(errors.New(fmt.Sprintf("pod [%s] container not ready ,traceInfo: [%s]-->[%v]", pod.Name, podStatusInfo, cond)), "")
			return pod, define.CreateInterruptError(define.InterruptDbStartFailed, errors.New(fmt.Sprintf("pod [%s] container not ready! trace: [%s][%s] ", pod.Name, cond.Message, podStatusInfo)))
		}
	}
	logger.Info("check containerReady and podReady", "name", pod.Name, "containerReady", containerReady, "podReady", podReady)

	pod = newPod
	return pod, nil
}

func GetPodContainerInfo(pod *corev1.Pod, logger logr.Logger) string {
	if pod == nil {
		return ""
	}
	phase := fmt.Sprintf("%v", pod.Status.Phase)
	statuses := pod.Status.ContainerStatuses
	if statuses == nil || len(statuses) < 1 {
		return phase
	}
	var statusMsg = []string{phase}
	for _, status := range statuses {
		msg := fmt.Sprintf("[%s] ready: %v, stauts: [%v]", status.Name, status.Ready, status.State)
		statusMsg = append(statusMsg, msg)
	}
	return strings.Join(statusMsg, "|")
}

func GetNode(nodeName string, logger logr.Logger) (*v1.Node, error) {
	coreClient, err := GetCorev1Client()
	if err != nil {
		logger.Error(err, "GetCorev1Client failed")
		return nil, err
	}
	return coreClient.Nodes().Get(nodeName, metav1.GetOptions{})
}

func GetNodes(logger logr.Logger) ([]v1.Node, error) {
	var result []v1.Node

	coreClient, err := GetCorev1Client()
	if err != nil {
		logger.Error(err, "GetCorev1Client failed")
		return nil, err
	}
	nodes, err := coreClient.Nodes().List(metav1.ListOptions{})
	if err != nil {
		logger.Error(err, "list nodes error")
		return nil, err
	}
	for _, node := range nodes.Items {
		result = append(result, node)
	}
	return result, nil
}

func SetWatchOption(object metav1.ObjectMeta, timeout int64) *metav1.ListOptions {
	objectSingleOption := metav1.SingleObject(object)
	objectSingleOption.TimeoutSeconds = &timeout
	return &objectSingleOption
}

func GetPodWatcher(pod *corev1.Pod, client corev1client.CoreV1Interface, timeout int64, logger logr.Logger) (watch.Interface, error) {
	opt := SetWatchOption(pod.ObjectMeta, timeout)
	if rv := GetPodWatchResourceVersion(pod.Name, pod.Namespace, client, logger); rv != "" {
		opt.ResourceVersion = rv
	}
	return client.Pods(pod.Namespace).Watch(*opt)
}

func GetPodWatchResourceVersion(name string, namespace string, client corev1client.CoreV1Interface, logger logr.Logger) string {
	nameSelector := fields.OneTermEqualSelector("metadata.name", name).String()
	list, err := client.Pods(namespace).List(metav1.ListOptions{
		FieldSelector: nameSelector,
	})
	if err != nil {
		logger.Error(err, "GetPodWatchResourceVersion failed", "name", name)
		return ""
	}
	return list.ResourceVersion
}

func WaitForPodDeleted(namespace, name string, ctx context.Context, logger logr.Logger) error {
	foundPod, err := GetPod(name, namespace, logger)
	if err != nil && apierrors.IsNotFound(err) {
		// not found,
		return nil
	} else if err != nil {
		// other error
		return err
	} else {
		coreclient, err := GetCorev1Client()
		if err != nil {
			logger.Error(err, "GetCorev1Client failed")
			return errors.Wrap(err, "get core v1 client error")
		}

		// Wait for the Pod to indicate Ready == True.
		watcher, err := GetPodWatcher(foundPod, coreclient, 60, logger)
		defer func() {
			if watcher != nil {
				watcher.Stop()
			}
		}()
		if err != nil {
			return errors.Wrap(err, "watch pod error")
		}

		for {
			select {
			case event := <-watcher.ResultChan():
				switch event.Type {
				case watch.Modified:
					continue
				case watch.Deleted:
					return nil
				default:
					err = errors.New(fmt.Sprintf("unexpected event type %s when wait for pod delete", string(event.Type)))
				}
			case <-ctx.Done():
				err = errors.New("cancelled when wait for pod delete.")
				logger.Error(err, "")
				return err
			}
		}
	}
}

func IsNodeReady(node *v1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == v1.NodeReady {
			return c.Status == v1.ConditionTrue
		}
	}
	return false
}

func GetNodeCondition(node *corev1.Node, conditionType corev1.NodeConditionType) *corev1.NodeCondition {
	for _, condition := range node.Status.Conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

func StringOfCondition(condition *corev1.NodeCondition) string {
	if condition == nil {
		return "nil"
	}
	return fmt.Sprintf("%s->%s: %s,%s,heatBeat:%v", condition.Type, condition.Status, condition.Reason, condition.Message, condition.LastHeartbeatTime)
}

func WaitForContainerReady(pod *corev1.Pod, containerName string, ctx context.Context, logger logr.Logger) (*corev1.Pod, error) {
	if podContainerReady(pod, containerName, logger) {
		return pod, nil
	}

	coreClient, err := GetCorev1Client()
	if err != nil {
		return pod, errors.Wrap(err, "get core v1 client error")
	}

	// Wait for the Pod to indicate Ready == True.
	podOpt := SetWatchOption(pod.ObjectMeta, 600)
	// 设置当前watch的resource 从头开始，确认pod曾经ready过.
	podOpt.ResourceVersion = pod.ResourceVersion
	watcher, err := coreClient.Pods(pod.Namespace).Watch(*podOpt)
	defer watcher.Stop()
	if err != nil {
		return pod, errors.Wrap(err, "watch pod error")
	}
	for {
		select {
		case event := <-watcher.ResultChan():
			switch event.Type {
			// 如果是 watch.Added event type, 继续执行下一个case, 并且不会判断case 表达式.
			case watch.Added:
				fallthrough
			case watch.Modified:
				pod = event.Object.(*corev1.Pod)
				if podContainerReady(pod, containerName, logger) {
					return pod, nil
				}
				continue
			default:
				err = errors.New(fmt.Sprintf("unexpected event type %s when wait for pod ready", string(event.Type)))
				logger.Error(err, "")
				return pod, err
			}
		case <-ctx.Done():
			err = errors.New("cancelled when wait for pod ready.")
			logger.Error(err, "")
			return pod, err
		}
	}
}

func IsPodSchedulerFailed(cond *corev1.PodCondition) bool {
	if cond == nil {
		return false
	}

	if cond.Type == corev1.PodScheduled &&
		cond.Status == corev1.ConditionTrue {
		//已调度成功
		return false
	}

	if cond.Type == corev1.PodScheduled && cond.Reason == corev1.PodReasonUnschedulable &&
		cond.Status == corev1.ConditionFalse {
		//调度失败
		return true
	}

	if cond.Type == corev1.PodScheduled &&
		cond.Status == corev1.ConditionFalse {
		//调度失败 ： 也认定为失败？
		return true
	}

	if cond.Type == corev1.PodReasonUnschedulable &&
		cond.Status == corev1.ConditionTrue {
		//疑似。。。。，理论上不存在
		return true
	}
	return false
}

func StringOfPodCondition(condition *corev1.PodCondition) string {
	if condition == nil {
		return "nil"
	}
	return fmt.Sprintf("%s->%s: %s,%s", condition.Type, condition.Status, condition.Reason, condition.Message)
}

func podContainerReady(pod *corev1.Pod, containerName string, logger logr.Logger) bool {
	for _, conStatus := range pod.Status.ContainerStatuses {
		logger.Info("pod container ready status", "podName", pod.Name, "containerStatus", conStatus.Name, "status", conStatus.Ready)
		if conStatus.Name == containerName && conStatus.Ready {
			return true
		}

	}
	return false
}
