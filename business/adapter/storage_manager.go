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

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/waitutil"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"

	"github.com/ApsaraDB/PolarDB-Stack-Common/define"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/retryutil"
)

type StorageSvcResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg,omitempty"`
}

type StorageSvcRequestRetry struct {
	logger         logr.Logger
	requestUrl     string
	method         string
	requestContent string
	timeoutSeconds time.Duration
	body           string
	times          int
}

func (r *StorageSvcRequestRetry) ConditionFunc() (bool, error) {
	logger := r.logger.WithValues("retry", r.times, "url", r.requestUrl)
	r.times++
	var err error
	r.body, err = request(r.method, r.requestUrl, r.requestContent, r.timeoutSeconds, logger)
	if err != nil {
		return false, err
	}
	return true, nil
}

func NewStorageManager(logger logr.Logger) *StorageManager {
	return &StorageManager{
		logger: logger.WithValues("component", "StorageManager"),
	}
}

type StorageManager struct {
	ip     string
	port   string
	logger logr.Logger
	inited bool
}

func (s *StorageManager) InitWithAddress(ip, port string) error {
	s.ip = ip
	s.port = port
	s.inited = true
	return nil
}

func (s *StorageManager) UseStorage(ctx context.Context, name, namespace, volumeId, resourceName, volumeType string, format bool) error {
	if err := s.initialize(); err != nil {
		return err
	}
	wfId, err := s.requestUseStorage(ctx, name, namespace, volumeId, resourceName, volumeType, format)
	if err != nil {
		return err
	}
	return waitutil.PollImmediateWithContext(ctx, 5*time.Second, 60*time.Second, func() (bool, error) {
		isReady, err := s.requestCheckReady(ctx, name, namespace, wfId)
		if err != nil {
			return false, err
		}
		if !isReady {
			return false, nil
		}
		return true, nil
	})
}

func (s *StorageManager) SetWriteLock(ctx context.Context, name, namespace, nodeId string) (*domain.StorageTopo, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}
	wfId, err := s.requestWriteLock(ctx, name, namespace, nodeId)
	if err != nil {
		return nil, err
	}
	var topoResult *domain.StorageTopo
	err = waitutil.PollImmediateWithContext(ctx, 5*time.Second, 60*time.Second, func() (bool, error) {
		var errInner error
		topoResult, errInner = s.requestTopo(ctx, name, namespace, wfId)
		if errInner != nil {
			return false, errInner
		}
		if topoResult != nil && topoResult.WriteNode != nil && topoResult.WriteNode.NodeId == nodeId {
			return true, nil
		}
		return false, nil
	})
	return topoResult, err
}

func (s *StorageManager) Expand(ctx context.Context, name, namespace, volumeId, resourceName string, volumeType int, reqSize int64) error {
	if err := s.initialize(); err != nil {
		return err
	}
	wfId, err := s.requestExpand(ctx, name, namespace, volumeId, resourceName, volumeType, reqSize)
	if err != nil {
		return err
	}

	err = waitutil.PollImmediateWithContext(ctx, 3*time.Second, 60*time.Second, func() (bool, error) {
		var isReady bool
		var errInner error
		isReady, errInner = s.requestExpandReady(ctx, name, namespace, wfId)
		if errInner != nil {
			return false, errInner
		}
		if isReady {
			return true, nil
		}
		return false, nil
	})
	return err
}

func (s *StorageManager) GetTopo(ctx context.Context, name, namespace string) (*domain.StorageTopo, error) {
	if err := s.initialize(); err != nil {
		return nil, err
	}
	return s.requestTopo(ctx, name, namespace, define.StorageDummyWorkflowId)
}

/**
 * @Description: 释放存储
 * @param ctx
 * @param name pvc名称
 * @param name pvc namespace
 * @param cluster 防止误释放其它集群使用的存储；为空则不校验存储是否被该集群使用，直接释放
 * @return error
 */
func (s *StorageManager) Release(ctx context.Context, name, namespace, clusterName string) error {
	if err := s.initialize(); err != nil {
		return err
	}
	if clusterName != "" {
		usedByClusterName, err := s.requestPvcUsedBy(ctx, name, namespace)
		if err != nil {
			if strings.Contains(err.Error(), "can not find") {
				return nil
			}
			return err
		}
		if usedByClusterName != clusterName {
			return errors.Errorf("used by other cluster %s", usedByClusterName)
		}
	}

	wfId, err := s.requestRelease(ctx, name, namespace)
	if err != nil {
		if strings.Contains(err.Error(), "can not find") {
			return nil
		}
		return err
	}
	return waitutil.PollImmediateWithContext(ctx, 5*time.Second, 60*time.Second, func() (bool, error) {
		isReady, err := s.requestCheckReady(ctx, name, namespace, wfId)
		if err != nil {
			return false, err
		}
		if !isReady {
			return false, nil
		}
		return true, nil
	})
}

func (s *StorageManager) requestWriteLock(ctx context.Context, name, namespace, nodeId string) (workflowId string, err error) {
	resp, err := s.doActionPost(ctx, "lock", map[string]string{
		"name":               name,
		"namespace":          namespace,
		"write_lock_node_id": nodeId,
	}, 10*time.Second)
	if err != nil {
		return "", err
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return "", errors.Wrapf(err, "unmarshal response error, %s", resp)
	}
	if workflowId, ok := result["workflow_id"]; !ok {
		return "", errors.Wrapf(err, "requst storage write lock service result: %s, workflowId field not found", resp)
	} else {
		return workflowId, nil
	}
}

func (s *StorageManager) requestTopo(ctx context.Context, name, namespace, workflowId string) (result *domain.StorageTopo, err error) {
	requestContent := url.Values{}
	requestContent.Add("name", name)
	requestContent.Add("namespace", namespace)
	requestContent.Add("workflowId", workflowId)
	resp, err := s.doActionGet(ctx, "topo", requestContent, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return result, errors.Wrapf(err, "unmarshal response error, %v", resp)
	}
	return
}

func (s *StorageManager) requestUseStorage(ctx context.Context, name, namespace, volumeId, resourceName, volumeType string, format bool) (workflowId string, err error) {
	resp, err := s.doActionPost(ctx, "use", map[string]interface{}{
		"namespace":   namespace,
		"name":        name,
		"resource_id": resourceName,
		"volume_id":   volumeId,
		"volume_type": volumeType,
		"need_format": format,
	}, 10*time.Second)
	if err != nil {
		return "", err
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return "", errors.Wrapf(err, "unmarshal response error, %v", resp)
	}
	if workflowId, ok := result["workflow_id"]; !ok {
		return "", errors.Wrapf(err, "requst storage use service result: %s, workflowId field not found", resp)
	} else {
		return workflowId, nil
	}
}

func (s *StorageManager) requestRelease(ctx context.Context, name, namespace string) (workflowId string, err error) {
	resp, err := s.doActionPost(ctx, "release", map[string]interface{}{
		"namespace": namespace,
		"name":      name,
	}, 10*time.Second)
	if err != nil {
		return "", err
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return "", errors.Wrapf(err, "unmarshal response error, %v", resp)
	}
	if workflowId, ok := result["workflow_id"]; !ok {
		return "", errors.Wrapf(err, "requst storage release service result: %s, workflowId field not found", resp)
	} else {
		return workflowId, nil
	}
}

func (s *StorageManager) requestPvcUsedBy(ctx context.Context, name, namespace string) (clusterName string, err error) {
	requestContent := url.Values{}
	requestContent.Add("name", name)
	requestContent.Add("namespace", namespace)
	resp, err := s.doActionGet(ctx, name, requestContent, 10*time.Second)
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return "", errors.Wrapf(err, "unmarshal response error, %v", resp)
	}
	if clusterName, ok := result["db_cluster_name"]; ok {
		return clusterName.(string), nil
	}
	return "", nil
}

func (s *StorageManager) requestCheckReady(ctx context.Context, name, namespace, workflowId string) (isReady bool, err error) {
	requestContent := url.Values{}
	requestContent.Add("name", name)
	requestContent.Add("namespace", namespace)
	requestContent.Add("workflowId", workflowId)
	resp, err := s.doActionGet(ctx, "ready", requestContent, 10*time.Second)
	if err != nil {
		return false, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return false, errors.Wrapf(err, "unmarshal response error, %v", resp)
	}
	isReady = result["is_ready"] == true
	return
}

func (s *StorageManager) requestExpand(ctx context.Context, name, namespace, volumeId, resourceName string, volumeType int, reqSize int64) (workflowId string, err error) {
	resp, err := s.doActionPost(ctx, "expand", map[string]interface{}{
		"namespace":   namespace,
		"name":        name,
		"resource_id": resourceName,
		"volume_id":   volumeId,
		"volume_type": volumeType,
		"req_size":    reqSize,
	}, 10*time.Second)
	if err != nil {
		return "", err
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return "", errors.Wrapf(err, "unmarshal response error, %s", resp)
	}
	if workflowId, ok := result["workflow_id"]; !ok {
		return "", errors.Wrapf(err, "requst storage expand service result: %s, workflowId field not found", resp)
	} else {
		return workflowId, nil
	}
}

func (s *StorageManager) requestExpandReady(ctx context.Context, name, namespace, workflowId string) (isReady bool, err error) {
	requestContent := url.Values{}
	requestContent.Add("name", name)
	requestContent.Add("namespace", namespace)
	requestContent.Add("workflowId", workflowId)
	resp, err := s.doActionGet(ctx, "expand", requestContent, 10*time.Second)
	if err != nil {
		return false, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return false, errors.Wrapf(err, "unmarshal response error, %v", resp)
	}
	status := result["status"].(float64)
	if status == 0 {
		isReady = true
	} else if status == 1 {
		isReady = false
	} else {
		isReady = false
		var errMsg string
		if errStr, ok := result["err_message"]; ok {
			errMsg = errStr.(string)
		} else {
			errMsg = fmt.Sprintf("request expand status error, response [%s]", resp)
		}
		err = errors.New(errMsg)
	}
	return
}

func (s *StorageManager) doActionGet(ctx context.Context, action string, params url.Values, timeoutSecond time.Duration) (string, error) {
	queryString := params.Encode()
	if queryString != "" {
		queryString = fmt.Sprintf("?%s", queryString)
	}
	requestUrl := fmt.Sprintf("http://%s:%s/pvcs/%s%s", s.ip, s.port, action, queryString)
	return doActionCommon(ctx, requestUrl, "GET", "", timeoutSecond, s.logger)
}

func (s *StorageManager) doActionPost(ctx context.Context, action string, params interface{}, timeoutSecond time.Duration) (string, error) {
	requestUrl := fmt.Sprintf("http://%s:%s/pvcs/%s", s.ip, s.port, action)
	requestContent, err := json.Marshal(params)
	if err != nil {
		s.logger.Error(err, "struct to map json.Marshal error")
		return "", err
	}
	return doActionCommon(ctx, requestUrl, "POST", string(requestContent), timeoutSecond, s.logger)
}

func doActionCommon(ctx context.Context, requestUrl string, method string, requestContent string, timeoutSeconds time.Duration, logger logr.Logger) (string, error) {
	var crr = &StorageSvcRequestRetry{
		logger:         logger,
		requestUrl:     requestUrl,
		method:         method,
		requestContent: requestContent,
		timeoutSeconds: timeoutSeconds,
	}
	err := retryutil.Retry(ctx, 2*time.Second, 3, crr)
	if err != nil {
		return "", err
	}
	return crr.body, err
}

func request(method string, url string, content string, timeoutSeconds time.Duration, logger logr.Logger) (string, error) {
	var bodyReader io.Reader
	if content != "" {
		bodyReader = strings.NewReader(content)
	}
	httpReq, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpClient := &http.Client{Timeout: timeoutSeconds * time.Second}
	logger.Info("1. request storage manager service", "url", url, "content", content)
	httpResp, err := httpClient.Do(httpReq)
	if err != nil {
		logger.Error(err, "3. request storage manager service error")
		return "", err
	}
	return handleResponse(httpResp, logger)
}

func handleResponse(resp *http.Response, logger logr.Logger) (string, error) {
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return string(body), err
	}
	logger.Info("2. request storage manager service succeed", "statusCode", resp.StatusCode, "body", string(body))
	if resp.StatusCode != 200 {
		return string(body), errors.New(fmt.Sprintf("{ 'statusCode': %d, 'body': '%s' }", resp.StatusCode, string(body)))
	}
	return string(body), nil
}

func (s *StorageManager) initialize() error {
	if s.inited {
		return nil
	}
	ip, port, err := getStorageServiceAddress(s.logger)
	if err != nil {
		return err
	}
	s.ip = ip
	s.port = port
	s.inited = true
	return nil
}

func getStorageServiceAddress(logger logr.Logger) (ip, port string, err error) {
	service, err := k8sutil.GetService(define.StorageServiceName, define.StorageServiceNamespace, logger)
	if err != nil {
		return "", "", err
	}
	ip = service.Spec.ClusterIP
	port = strconv.Itoa(int(service.Spec.Ports[0].Port))
	return
}
