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


package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"

	mgr "gitlab.alibaba-inc.com/polar-as/polar-common-domain/manager"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/define"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/klogr"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/waitutil"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/k8sutil"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/business/domain"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/retryutil"
	v1 "k8s.io/api/core/v1"
)

func NewClusterManagerClient(logger logr.Logger) *ClusterManagerClient {
	return &ClusterManagerClient{
		logger: logger.WithValues("component", "ClusterManagerClient"),
	}
}

type ClusterManagerClient struct {
	IP                 string
	Port               string
	DbClusterNamespace string
	DbClusterName      string
	inited             bool
	logger             logr.Logger
}

var cmNotInitError = errors.New("cluster manager client not init.")

func (m *ClusterManagerClient) InitWithLocalDbCluster(ctx context.Context, dbClusterNamespace, dbClusterName string, waitCmReady bool) error {
	m.DbClusterName = dbClusterName
	m.DbClusterNamespace = dbClusterNamespace
	if m.logger == nil {
		m.logger = klogr.New()
	}
	m.logger = m.logger.WithValues("cluster", dbClusterNamespace+"/"+dbClusterName)
	_, err := GetCmDeployment(dbClusterName, dbClusterNamespace)
	if err != nil {
		return err
	}
	cmIp, cmPort, err := getLocalCmIpPort(dbClusterName, dbClusterNamespace, m.logger, waitCmReady, ctx)
	if err != nil {
		return err
	}
	m.logger = m.logger.WithValues("IP", cmIp, "Port", cmPort)
	m.IP = cmIp
	m.Port = cmPort
	m.inited = true
	return nil
}

/**
 * @Description: 设置跨域主集群
 * @param ctx
 * @return error
 */
func (m *ClusterManagerClient) SetLeader(ctx context.Context) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionPost(ctx, "global_set_leader", map[string]interface{}{}, 10*time.Second)
	return err
}

/**
 * @Description: 取消跨域主集群
 * @param ctx
 * @return error
 */
func (m *ClusterManagerClient) SetFollower(ctx context.Context) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionPost(ctx, "global_set_follower", map[string]interface{}{}, 10*time.Second)
	return err
}

/**
 * @Description: 添加DataMax节点
 * @param ctx
 * @param id
 * @param ip
 * @param podName
 * @param port
 * @return error
 */
func (m *ClusterManagerClient) AddDataMax(ctx context.Context, id, ip, podName string, port int) error {
	if !m.inited {
		return cmNotInitError
	}
	clientIP, err := k8sutil.GetClientIP(ip, m.logger)
	if err != nil {
		return err
	}
	//err = m.DeleteDataMax(ctx, id)
	//if err != nil && isUsingError(err.Error()) {
	//	return m.UpdateDataMax(ctx, id, ip, podName, port)
	//}
	//if err != nil && !isNotExistError(err.Error()) {
	//	return err
	//}
	_, err = m.doActionPost(ctx, "global_add_datamax", map[string]interface{}{
		"id":              id,
		"ip":              clientIP,
		"pod_name":        podName,
		"port":            strconv.Itoa(port),
		"use_node_driver": "false",
	}, 10*time.Second)
	if err != nil && isAlreadyExistError(err.Error()) {
		return m.UpdateDataMax(ctx, id, ip, podName, port)
	}

	return err
}

/**
 * @Description: 更新DataMax节点
 * @param ctx
 * @param id
 * @param ip
 * @param podName
 * @param port
 * @return error
 */
func (m *ClusterManagerClient) UpdateDataMax(ctx context.Context, id, ip, podName string, port int) error {
	if !m.inited {
		return cmNotInitError
	}
	clientIP, err := k8sutil.GetClientIP(ip, m.logger)
	if err != nil {
		return err
	}
	_, err = m.doActionPost(ctx, "global_update_datamax", map[string]interface{}{
		"id":              id,
		"ip":              clientIP,
		"pod_name":        podName,
		"port":            strconv.Itoa(port),
		"use_node_driver": "false",
	}, 10*time.Second)
	return err
}

/**
 * @Description: 删除DataMax节点
 * @param ctx
 * @param id
 * @return error
 */
func (m *ClusterManagerClient) DeleteDataMax(ctx context.Context, id string) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionPost(ctx, "global_remove_datamax", map[string]interface{}{
		"id":              id,
		"use_node_driver": "false",
	}, 10*time.Second)
	if err != nil && isNotExistError(err.Error()) {
		return nil
	}
	return err
}

/**
 * @Description: 添加集群节点
 * @param ctx
 * @param ctx
 * @param id
 * @param ip
 * @param port
 * @return error
 */
func (m *ClusterManagerClient) AddCluster(ctx context.Context, id, ip string, port int, isClientIP bool) error {
	if !m.inited {
		return cmNotInitError
	}
	clientIP := ip
	var err error
	if !isClientIP {
		clientIP, err = k8sutil.GetClientIP(ip, m.logger)
		if err != nil {
			return err
		}
	}
	err = m.DeleteCluster(ctx, id)
	if err != nil && isUsingError(err.Error()) {
		m.logger.Info(fmt.Sprintf("ignore global_add_cluster, %+v", err))
		return nil
	}
	if err != nil && !isNotExistError(err.Error()) {
		return err
	}
	type EndPoint struct {
		Host string `json:"host,omitempty"`
		Port string `json:"port,omitempty"`
	}
	type CmClusterSpec struct {
		ID        string     `json:"id"`
		Endpoints []EndPoint `json:"endpoints"`
	}
	_, err = m.doActionPost(ctx, "global_add_cluster", CmClusterSpec{
		ID: id,
		Endpoints: []EndPoint{
			{
				Host: clientIP,
				Port: strconv.Itoa(port),
			},
		},
	}, 10*time.Second)
	if err != nil && isAlreadyExistError(err.Error()) {
		return m.UpdateCluster(ctx, id, ip, port, isClientIP)
	}
	return err
}

/**
 * @Description: 更新集群节点
 * @param ctx
 * @param id
 * @param ip
 * @param port
 * @return error
 */
func (m *ClusterManagerClient) UpdateCluster(ctx context.Context, id, ip string, port int, isClientIP bool) error {
	if !m.inited {
		return cmNotInitError
	}
	clientIP := ip
	var err error
	if !isClientIP {
		clientIP, err = k8sutil.GetClientIP(ip, m.logger)
		if err != nil {
			return err
		}
	}
	_, err = m.doActionPost(ctx, "global_update_cluster", map[string]interface{}{
		"id":   id,
		"ip":   clientIP,
		"port": strconv.Itoa(port),
	}, 10*time.Second)
	return err
}

/**
 * @Description: 删除集群节点
 * @param ctx
 * @param id
 * @return error
 */
func (m *ClusterManagerClient) DeleteCluster(ctx context.Context, id string) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionPost(ctx, "global_remove_cluster", map[string]interface{}{
		"id": id,
	}, 10*time.Second)
	if err != nil && isNotExistError(err.Error()) {
		return nil
	}
	return err
}

/**
 * @Description: 创建拓扑关系
 * @param ctx
 * @param upStreamId 上游节点ID
 * @param downStreamId 下游节点ID
 * @param copyType 复制关系类型
 * @return error
 */
func (m *ClusterManagerClient) AddTopologyEdge(ctx context.Context, upStreamId, downStreamId string, copyType domain.CopyType) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionPost(ctx, "global_add_topology_edge", map[string]interface{}{
		"up_stream_id":   upStreamId,
		"down_stream_id": downStreamId,
		"type":           string(copyType),
	}, 10*time.Second)
	return err
}

/**
 * @Description: 删除拓扑关系
 * @param ctx
 * @param upStreamId 上游节点ID
 * @param downStreamId 下游节点ID
 * @return error
 */
func (m *ClusterManagerClient) DeleteTopologyEdge(ctx context.Context, upStreamId, downStreamId string) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionPost(ctx, "global_remove_topology_edge", map[string]interface{}{
		"up_stream_id":   upStreamId,
		"down_stream_id": downStreamId,
	}, 10*time.Second)
	// 删除拓扑会检查上下游，上下游不存在报错也会包含not exist。
	if err != nil && isNotExistError(err.Error()) {
		return nil
	}
	return err
}

/**
 * @Description: 设置standby订阅DataMax
 * @param ctx
 * @param id 集群ID
 * @param ip 上游 client IP
 * @param port 上游 端口
 * @param copyType 复制类型
 * @param upStreamId
 * @return error
 */
func (m *ClusterManagerClient) DemoteStandby(ctx context.Context, id, ip string, port int, copyType domain.CopyType, upStreamId string) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionPost(ctx, "global_demote", map[string]interface{}{
		"id": id,
		"up_stream": map[string]string{
			"ip":   ip,
			"port": strconv.Itoa(port),
			"id":   upStreamId,
		},
		"type":            string(copyType),
		"use_node_driver": "false",
	}, 10*time.Second)
	return err
}

/**
 * @Description: standby promote
 * @param ctx
 * @return error
 */
func (m *ClusterManagerClient) PromoteStandby(ctx context.Context) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionGet(ctx, "global_promote", nil, 10*time.Second)
	return err
}

/**
 * @Description: 禁用HA
 * @param ctx
 * @return error
 */
func (m *ClusterManagerClient) DisableHA(ctx context.Context) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionPost(ctx, "enable", map[string]interface{}{
		"enable": 0,
	}, 10*time.Second)
	return err
}

/**
 * @Description: 启用HA
 * @param ctx
 * @return error
 */
func (m *ClusterManagerClient) EnableHA(ctx context.Context) error {
	if !m.inited {
		return cmNotInitError
	}
	_, err := m.doActionPost(ctx, "enable", map[string]interface{}{
		"enable": 1,
	}, 10*time.Second)
	return err
}

/**
 * @Description: 添加实例
 * @param ctx
 * @param id
 * @param ip
 * @param port
 * @return error
 */
func (m *ClusterManagerClient) AddIns(ctx context.Context, id, ip, role, sync, hostName string, port int, isClientIP bool) error {
	if !m.inited {
		return cmNotInitError
	}
	clientIP := ip
	var err error
	if !isClientIP {
		clientIP, err = k8sutil.GetClientIP(ip, m.logger)
		if err != nil {
			return err
		}
	}
	type CmInsSpec struct {
		IP            string `json:"ip"`
		Port          string `json:"port"`
		Type          string `json:"type"`
		Sync          string `json:"sync"`
		PodName       string `json:"pod_name"`
		UseNodeDriver string `json:"use_node_driver"`
		HostName      string `json:"host_name,omitempty"`
	}
	_, err = m.doActionPost(ctx, "add_ins", CmInsSpec{
		clientIP,
		strconv.Itoa(port),
		role,
		sync,
		id,
		"false",
		hostName,
	}, 10*time.Second)
	if err != nil && isAlreadyExistError(err.Error()) {
		return nil
	}
	return err
}

/**
 * @Description: 删除实例
 * @param ctx
 * @param id
 * @param ip
 * @param port
 * @return error
 */
func (m *ClusterManagerClient) RemoveIns(ctx context.Context, id, ip, role, sync, hostName string, port int, isClientIP bool) error {
	if !m.inited {
		return cmNotInitError
	}
	clientIP := ip
	var err error
	if !isClientIP {
		clientIP, err = k8sutil.GetClientIP(ip, m.logger)
		if err != nil {
			return err
		}
	}

	type CmInsSpec struct {
		IP            string `json:"ip"`
		Port          string `json:"port"`
		Type          string `json:"type"`
		Sync          string `json:"sync"`
		PodName       string `json:"pod_name"`
		UseNodeDriver string `json:"use_node_driver"`
		HostName      string `json:"host_name,omitempty"`
	}
	_, err = m.doActionPost(ctx, "remove_ins", CmInsSpec{
		clientIP,
		strconv.Itoa(port),
		role,
		sync,
		id,
		"false",
		hostName,
	}, 10*time.Second)
	if err != nil && !isNotExistError(err.Error()) {
		return err
	}
	return nil
}

/**
 * @Description: 读写切换
 * @param ctx
 * @param oriRwIp
 * @param oriRoIp
 * @param port
 * @return error
 */
func (m *ClusterManagerClient) Switchover(ctx context.Context, oriRwIp, oriRoIp, port string, isClientIP bool) error {
	if !m.inited {
		return cmNotInitError
	}
	rwClientIP := oriRwIp
	roClientIP := oriRoIp
	var err error
	if !isClientIP {
		rwClientIP, err = k8sutil.GetClientIP(oriRwIp, m.logger)
		if err != nil {
			return err
		}
		roClientIP, err = k8sutil.GetClientIP(oriRoIp, m.logger)
		if err != nil {
			return err
		}
	}
	insStatus, err := m.WaitForStartingCompleted(ctx, roClientIP, port, 5*time.Minute)
	if err != nil {
		if insStatus != nil {
			return errors.Wrap(err, fmt.Sprintf("RO %s status is %s and cannot be promoted to RW.", insStatus.PodName, insStatus.Phase))
		} else {
			return errors.Wrap(err, fmt.Sprintf("RO %s:%s status is not found from cm status metadata.", oriRoIp, port))
		}
	}

	_, err = m.doAction(ctx, fmt.Sprintf("http://%s:%s/v1/switchover", m.IP, m.Port), map[string]interface{}{
		"from":  fmt.Sprintf("%s:%s", rwClientIP, port),
		"to":    fmt.Sprintf("%s:%s", roClientIP, port),
		"force": 1,
	}, 10*time.Second)

	return err
}

/**
 * @Description: 获取集群状态
 * @param ctx
 * @return error
 */
func (m *ClusterManagerClient) GetClusterStatus(ctx context.Context) (*domain.ClusterStatus, error) {
	if !m.inited {
		return nil, cmNotInitError
	}
	requestUrl := fmt.Sprintf("http://%s:%s/v1/status?type=visual", m.IP, m.Port)
	body, err := requestCm("GET", requestUrl, "", 5*time.Second, m.logger, false, nil)
	if err != nil {
		return nil, err
	}
	var statusResp = domain.ClusterStatus{}
	if err = json.Unmarshal([]byte(body), &statusResp); err != nil {
		m.logger.Error(err, "json.Unmarshal failed", "body", body)
		return nil, err
	}
	return &statusResp, nil
}

func (m *ClusterManagerClient) EnsureAffinity(ctx context.Context, clusterNamespace, clusterName, rwHostName string) error {
	cmPodList, err := getCmPods(clusterName, clusterNamespace, m.logger)
	if err != nil {
		return err
	}
	if cmPodList == nil || cmPodList.Items == nil {
		return nil
	}
	cmNeedDelete := len(cmPodList.Items) > 0 && rwHostName == cmPodList.Items[0].Spec.NodeName
	if cmNeedDelete {
		oldCmName := cmPodList.Items[0].Name
		err := mgr.GetSyncClient().Delete(context.TODO(), &cmPodList.Items[0])
		if !apierrors.IsNotFound(err) {
			m.logger.Error(err, "get cm %s pod error")
			return err
		}
		return waitForNewCmPodReady(clusterName, clusterNamespace, oldCmName, m.logger, ctx)
	}
	return nil
}

func (m *ClusterManagerClient) UpgradeVersion(ctx context.Context, clusterNamespace, clusterName, image string) error {
	return upgradeCmDeploy(clusterName, clusterNamespace, image, m.logger)
}

func (m *ClusterManagerClient) WaitForStartingCompleted(ctx context.Context, clientIp, port string, timeout time.Duration) (insStatus *domain.InsStatus, err error) {
	endpoint := fmt.Sprintf("%s:%s", clientIp, port)
	timeoutBeginTime := time.Now()
	err = waitutil.PollInfinite(ctx, 5*time.Second, func() (bool, error) {
		if timeoutBeginTime.Add(timeout).Before(time.Now()) {
			return false, errors.New("wait for starting up completed timeout.")
		}
		status, err := m.GetClusterStatus(ctx)
		if err != nil {
			return false, nil
		}
		if status.Rw.Endpoint == endpoint {
			insStatus = &status.Rw
		} else {
			for _, ro := range status.Ro {
				if ro.Endpoint == endpoint {
					insStatus = &ro
				}
			}
		}
		if insStatus == nil {
			return false, nil
		}
		if insStatus.Phase == define.EnginePhasePending || insStatus.Phase == define.EnginePhaseStarting {
			timeoutBeginTime = time.Now()
			return false, nil
		} else if insStatus.Phase != define.EnginePhaseRunning {
			return false, errors.New("ro status error")
		}
		return true, nil
	})
	return
}

func (m *ClusterManagerClient) DoActionCommon(ctx context.Context, requestUrl string, method string, requestContent string, timeoutSeconds time.Duration) (string, error) {
	return m.doActionCommon(ctx, requestUrl, method, requestContent, timeoutSeconds)
}

func getLocalCmIpPort(dbClusterName, dbClusterNamespace string, logger logr.Logger, waitCmReady bool, ctx context.Context) (ip, port string, err error) {
	selectCMPod, err := getCmRunningPod(dbClusterName, dbClusterNamespace, logger, waitCmReady, ctx)
	if err != nil {
		return "", "", err
	}
	cmPort := selectCMPod.Labels["port"]
	foundNode, err := k8sutil.GetNode(selectCMPod.Spec.NodeName, logger)
	if err != nil {
		return "", "", err
	}
	cmIp := ""
	for _, address := range foundNode.Status.Addresses {
		if v1.NodeInternalIP == address.Type {
			cmIp = address.Address
		}
	}
	if cmIp == "" {
		return "", "", errors.New(fmt.Sprintf("%s can't find the node ip", foundNode.Name))
	}

	return cmIp, cmPort, nil
}

func getCmRunningPod(dbClusterName, dbClusterNamespace string, logger logr.Logger, waitCmReady bool, ctx context.Context) (*v1.Pod, error) {
	var cmPod *v1.Pod
	var err error
	if !waitCmReady {
		cmPod, err = getRunningCmPod(dbClusterName, dbClusterNamespace, logger)
	} else {
		waitutil.PollImmediateWithContext(ctx, 2*time.Second, 3*time.Minute, func() (bool, error) {
			cmPod, err = getRunningCmPod(dbClusterName, dbClusterNamespace, logger)
			if err == nil && cmPod != nil {
				return true, nil
			}
			return false, nil
		})
	}
	return cmPod, err
}

func waitForNewCmPodReady(clusterName, clusterNamespace, oldCmName string, logger logr.Logger, ctx context.Context) error {
	return waitutil.PollImmediateWithContext(ctx, 2*time.Second, 3*time.Minute, func() (bool, error) {
		cmPod, err := getRunningCmPod(clusterName, clusterNamespace, logger)
		if err == nil && cmPod != nil {
			return true, nil
		}
		if cmPod.Name != oldCmName {
			logger.Info("[%s] new cm node: [%s]", clusterName, cmPod.Spec.NodeName)
			return true, nil
		}
		return false, nil
	})
}

func getRunningCmPod(dbClusterName, dbClusterNamespace string, logger logr.Logger) (*v1.Pod, error) {
	podList, err := getCmPods(dbClusterName, dbClusterNamespace, logger)
	if err != nil {
		return nil, err
	}
	if podList == nil {
		return nil, errors.New("cm pod is not found.")
	}
	var podNameList []string
	var runningPodList []v1.Pod
	for _, pod := range podList.Items {
		if (pod.Status.Phase == v1.PodRunning || pod.Status.Phase == v1.PodPending) && pod.DeletionTimestamp == nil {
			podNameList = append(podNameList, pod.Name)
			runningPodList = append(runningPodList, pod)
		}
	}
	podCount := len(podNameList)
	msg := fmt.Sprintf("cm running pod count: %d, [%s]", podCount, strings.Join(podNameList, ","))
	logger.Info(msg)
	if len(runningPodList) == 1 && runningPodList[0].Status.Phase == v1.PodRunning {
		return &runningPodList[0], nil
	}
	return nil, errors.New(msg)
}

func getCmPods(dbClusterName, dbClusterNamespace string, logger logr.Logger) (*v1.PodList, error) {
	return k8sutil.GetPodsByLabel("app="+dbClusterName+".cm", dbClusterNamespace, logger)
}

func upgradeCmDeploy(dbClusterName, dbClusterNamespace, cmImage string, logger logr.Logger) error {
	deployment, err := GetCmDeployment(dbClusterName, dbClusterNamespace)
	if err != nil {
		return err
	}

	for index, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			if container.Image == cmImage {
				return nil
			}
			deployment.Spec.Template.Spec.Containers[index].Image = cmImage
			deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
			deployment.Spec.Strategy.RollingUpdate = nil
		}
	}

	if err := mgr.GetSyncClient().Update(context.TODO(), deployment); err != nil {
		logger.Error(err, "upgrade cluster manager image failed")
		return err
	}
	return nil
}

func GetCmDeployment(dbClusterName string, dbClusterNamespace string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	if err := mgr.GetSyncClient().Get(context.TODO(), types.NamespacedName{Namespace: dbClusterNamespace, Name: dbClusterName + define.ClusterManagerNameSuffix}, deployment); err != nil {
		return nil, err
	}
	return deployment, nil
}

func waitForCmDeleted(dbClusterName, dbClusterNamespace string, logger logr.Logger, ctx context.Context) error {
	err := waitutil.PollImmediateWithContext(ctx, time.Second, 3*time.Minute, func() (bool, error) {
		cmPods, err := getCmPods(dbClusterName, dbClusterNamespace, logger)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				logger.Error(err, "get cm %s pod error")
				return false, nil
			}
		} else {
			if cmPods != nil && len(cmPods.Items) > 0 {
				return false, nil
			}
		}
		logger.Info("delete cm pod succeed")
		return true, nil
	})
	return err
}

func (m *ClusterManagerClient) doAction(ctx context.Context, requestUrl string, params map[string]interface{}, timeoutSecond time.Duration) (string, error) {
	jsonData, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	requestContent := string(jsonData)
	return m.doActionCommon(ctx, requestUrl, "POST", requestContent, timeoutSecond)
}

func (m *ClusterManagerClient) doActionCommon(ctx context.Context, requestUrl string, method string, requestContent string, timeoutSeconds time.Duration) (string, error) {
	crr := &CmRequestRetry{
		requestUrl:     requestUrl,
		method:         method,
		requestContent: requestContent,
		timeoutSeconds: timeoutSeconds,
		logger:         m.logger,
	}
	err := retryutil.Retry(ctx, 2*time.Second, 3, crr)
	if err != nil {
		return "", err
	}
	return crr.body, err
}

func (m *ClusterManagerClient) doActionGet(ctx context.Context, action string, params url.Values, timeoutSecond time.Duration) (string, error) {
	var queryString string
	if params != nil {
		queryString = params.Encode()
	}
	if queryString != "" {
		queryString = fmt.Sprintf("?%s", queryString)
	}
	requestUrl := fmt.Sprintf("http://%s:%s/v1/%s%s", m.IP, m.Port, action, queryString)
	return m.doActionCommon(ctx, requestUrl, "GET", "", timeoutSecond)
}

func (m *ClusterManagerClient) doActionPost(ctx context.Context, action string, params interface{}, timeoutSecond time.Duration) (string, error) {
	requestUrl := fmt.Sprintf("http://%s:%s/v1/%s", m.IP, m.Port, action)
	requestContent, err := json.Marshal(params)
	if err != nil {
		m.logger.Error(err, "doActionPost, struct to map json.Marshal error")
		return "", err
	}
	return m.doActionCommon(ctx, requestUrl, "POST", string(requestContent), timeoutSecond)
}

type CmResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg,omitempty"`
}

type CmRequestRetry struct {
	requestUrl     string
	method         string
	requestContent string
	timeoutSeconds time.Duration
	body           string
	logger         logr.Logger
}

func (r *CmRequestRetry) ConditionFunc() (bool, error) {
	var err error
	r.body, err = requestCm(r.method, r.requestUrl, r.requestContent, r.timeoutSeconds, r.logger, true, r.parseCmResponse)
	if err != nil {
		r.logger.Error(err, "response error")
		if isAlreadyExistError(r.body) || isNotExistError(r.body) || isUsingError(r.body) {
			return true, err
		}
		return false, err
	}
	return true, nil
}

func requestCm(method string, url string, content string, timeoutSeconds time.Duration, log logr.Logger, parseHaResp bool, bodyParser func(body string) (string, error)) (string, error) {
	logger := log.WithValues("method", method, "url", url, "content", content)
	logger.Info("begin request cm service")
	var bodyReader io.Reader
	if content != "" {
		bodyReader = strings.NewReader(content)
	}
	httpReq, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		logger.Error(err, "http.NewRequest error")
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: timeoutSeconds * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		logger.Error(err, "request cm service error")
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	result := string(body)
	if err == nil {
		statusCode := resp.StatusCode
		if statusCode != 200 {
			err = errors.New(fmt.Sprintf("http code: %d, body: [%s]", statusCode, string(body)))
		} else if parseHaResp {
			result, err = bodyParser(result)
		}
	}
	logger.Info("request cm service done", "response", result)
	return result, err
}

func (r *CmRequestRetry) parseCmResponse(body string) (string, error) {
	response := &CmResponse{}
	if err := json.Unmarshal([]byte(body), response); err != nil {
		return body, errors.Wrapf(err, "error unmarshal cluster manager response, body: [%s]", body)
	}
	if response.Code != 200 {
		return body, errors.New(fmt.Sprintf("body: [%s]", body))
	}
	return body, nil
}

func isAlreadyExistError(msg string) bool {
	return strings.Contains(msg, "already exist")
}

func isNotExistError(msg string) bool {
	return strings.Contains(msg, "not exist")
}

func isUsingError(msg string) bool {
	return strings.Contains(msg, "still in edge")
}
