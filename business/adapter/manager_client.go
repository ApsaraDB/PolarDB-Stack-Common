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
	"regexp"
	"strings"
	"time"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/waitutil"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/business/domain"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	errors2 "k8s.io/apimachinery/pkg/api/errors"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/k8sutil"
)

type IEnvGetStrategy interface {
	Load(domainModel interface{}, ins *domain.DbIns) error
	GetInstallEngineEnvirons(ctx context.Context) (string, error)
	GetStartEngineEnvirons() (string, error)
	GetSetupLogAgentEnvirons() (string, error)
	GetGracefulStopEngineEnvirons() (string, error)
	GetCreateEngineAccountEnvirons() ([]string, error)
	GetUnLockEngineEnvirons() (string, error)
	GetLockEngineEnvirons() (string, error)
	GetUpdateEngineParamsEnvirons(engineParams map[string]string) (string, error)
	GetGrowStorageEnvirons() (string, error)
	GetPrimarySystemIdentifier() (string, error)
	GetPodName() (string, string)
	GetCheckHealthEnvirons() (string, error)
	GetCreateReplicationSlotEnvirons(string) (string, error)
	GetInstallationProgressEnvirons() (string, error)
}

func NewManagerClient(strategy IEnvGetStrategy, logger logr.Logger) *ManagerClient {
	return &ManagerClient{
		EnvGetStrategy: strategy,
		logger:         logger.WithValues("component", "ManagerClient"),
	}
}

type ManagerClient struct {
	EnvGetStrategy IEnvGetStrategy
	PodName        string
	PodNamespace   string
	inited         bool
	domainModel    interface{}
	logger         logr.Logger
}

func (m *ManagerClient) Init(domainModel interface{}) {
	m.domainModel = domainModel
}

func (m *ManagerClient) SetIns(ins *domain.DbIns) error {
	if err := m.loadStrategy(m.domainModel, ins); err != nil {
		return err
	}
	podName, podNs := m.EnvGetStrategy.GetPodName()
	m.PodName = podName
	m.PodNamespace = podNs
	m.inited = true
	m.logger = m.logger.WithValues("podName", m.PodName)
	return nil
}

var notInitError = errors.New("manager client not init.")

/**
 * @Description: 安装引擎
 * @return error
 */
func (m *ManagerClient) InstallEngine(ctx context.Context) error {
	if !m.inited {
		return notInitError
	}
	envs, err := m.EnvGetStrategy.GetInstallEngineEnvirons(ctx)
	if err != nil {
		return errors.Wrap(err, "GetInstallEngineEnvirons error")
	}
	return m.ExecCmdInPod(ctx, envs)
}

/**
 * @Description: 创建UE所需元数据
 * @return error
 */
func (m *ManagerClient) SetupLogAgent(ctx context.Context) error {
	if !m.inited {
		return notInitError
	}
	envs, err := m.EnvGetStrategy.GetSetupLogAgentEnvirons()
	if err != nil {
		return errors.Wrap(err, "GetSetupLogAgentEnvirons error")
	}
	return m.ExecCmdInPod(ctx, envs)
}

/**
 * @Description: 启动引擎
 * @return error
 */
func (m *ManagerClient) StartEngine(ctx context.Context) error {
	if !m.inited {
		return notInitError
	}
	envs, err := m.EnvGetStrategy.GetStartEngineEnvirons()
	if err != nil {
		return errors.Wrap(err, "GetStartEngineEnvirons error")
	}
	return m.ExecCmdInPod(ctx, envs)
}

/**
 * @Description: 安全停止引擎
 * @return error
 */
func (m *ManagerClient) GracefulStopEngine(ctx context.Context) error {
	if !m.inited {
		return notInitError
	}
	envs, err := m.EnvGetStrategy.GetGracefulStopEngineEnvirons()
	if err != nil {
		return errors.Wrap(err, "GetGracefulStopEngineEnvirons error")
	}
	// 关机场景，执行肯定失败，所以超时时间不能设置太长
	_, _, err = m.ExecCmdInPodBase(ctx, envs, true, 10*time.Second)
	if err != nil {
		return errors.Wrap(err, "exec cmd in pod error")
	}
	return nil
}

/**
 * @Description: 强制停止引擎，kill 1
 * @receiver m
 * @return error
 */
func (m *ManagerClient) ForceStopEngine(ctx context.Context) error {
	if !m.inited {
		return notInitError
	}

	command := []string{"bash", "-c", "kill 1"}
	_, _, err := k8sutil.ExecInPod(command, "manager", m.PodName, m.PodNamespace, m.logger)
	if err != nil {
		return errors.Wrap(err, "exec kill 1 in pod error")
	}
	return nil
}

/**
 * @Description: 重启引擎
 * @return error
 */
func (m *ManagerClient) RestartEngine(ctx context.Context) error {
	var err error
	if err = m.GracefulStopEngine(ctx); err != nil {
		return err
	}

	foundPod, err := k8sutil.GetPod(m.PodName, m.PodNamespace, m.logger)
	if err != nil && errors2.IsNotFound(err) {
		return errors.Wrap(err, "pod not exist in RestartEngine")
	} else if err != nil {
		return errors.Wrap(err, "get pod error in RestartEngine")
	}

	if err = m.ForceStopEngine(ctx); err != nil {
		return err
	}

	if foundPod, err = k8sutil.WaitForPodReady(foundPod, false, ctx, m.logger); err != nil {
		return err
	}

	if err := m.StartEngine(ctx); err != nil {
		return err
	}

	return nil
}

/**
 * @Description: 锁定引擎
 * @return error
 */
func (m *ManagerClient) LockEngine(ctx context.Context) error {
	if !m.inited {
		return notInitError
	}
	envs, err := m.EnvGetStrategy.GetLockEngineEnvirons()
	if err != nil {
		return errors.Wrap(err, "GetLockEngineEnvirons error")
	}
	return m.ExecCmdInPod(ctx, envs)
}

/**
 * @Description: 创建账户
 * @return error
 */
func (m *ManagerClient) CreateEngineAccount(ctx context.Context) error {
	if !m.inited {
		return notInitError
	}
	envsList, err := m.EnvGetStrategy.GetCreateEngineAccountEnvirons()
	if err != nil {
		return errors.Wrap(err, "GetCreateEngineAccountEnvirons error")
	}
	for _, envs := range envsList {
		if err := m.ExecCmdInPod(ctx, envs); err != nil {
			return err
		}
	}
	return nil
}

/**
 * @Description: 更新引擎参数
 * @return error
 */
func (m *ManagerClient) UpdateEngineParams(ctx context.Context, engineParams map[string]string) error {
	if !m.inited {
		return notInitError
	}
	envs, err := m.EnvGetStrategy.GetUpdateEngineParamsEnvirons(engineParams)
	if err != nil {
		return errors.Wrap(err, "GetUpdateEngineParamsEnvirons error")
	}
	return m.ExecCmdInPod(ctx, envs)
}

/**
 * @Description: 存储扩容
 * @return error
 */
func (m *ManagerClient) GrowStorage(ctx context.Context) error {
	if !m.inited {
		return notInitError
	}
	envs, err := m.EnvGetStrategy.GetGrowStorageEnvirons()
	if err != nil {
		return errors.Wrap(err, "GetGrowStorageEnvirons error")
	}
	return m.ExecCmdInPod(ctx, envs)
}

/**
 * @Description: 健康检查
 * @receiver m
 * @return err
 */
func (m *ManagerClient) CheckHealth(ctx context.Context) *domain.HealthCheckError {
	if !m.inited {
		return domain.CreateHealthError(domain.HealthErrorUnKnown, "manager client is not init", notInitError)
	}
	envs, err := m.EnvGetStrategy.GetCheckHealthEnvirons()
	if err != nil {
		return domain.CreateHealthError(domain.HealthErrorUnKnown, "GetCheckHealthEnvirons error", err)
	}
	_, stdErr, err := m.ExecCmdInPodBase(ctx, envs, false, time.Second)
	if err != nil {
		if strings.Contains(err.Error(), "container not found") ||
			strings.Contains(err.Error(), "pod does not exist") {
			// unable to upgrade connection: container not found ("manager")
			m.logger.Error(err, "container not found, retry")
			return domain.CreateHealthError(domain.HealthErrorContainerNotFound, "container not found", err)
		} else {
			return domain.CreateHealthError(domain.HealthErrorUnKnown, "exec command in pod error", err)
		}
	} else if strings.Contains(stdErr, "the database system is starting up") {
		// engine正在启动中，可有由于回放慢，导致耗时很长，此时无需超时，一直等待
		return domain.CreateHealthError(domain.HealthErrorStartingUp, "the database system is starting up", err)
	}
	return nil
}

/**
 * @Description: 获取PrimarySystemIdentifier
 * @receiver m
 * @return err
 */
func (m *ManagerClient) GetPrimarySystemIdentifier(ctx context.Context) (string, error) {
	if !m.inited {
		return "", notInitError
	}
	envs, err := m.EnvGetStrategy.GetPrimarySystemIdentifier()
	if err != nil {
		return "", errors.Wrap(err, "GetPrimarySystemIdentifier error")
	}
	stdout, _, err := m.ExecCmdInPodBase(ctx, envs, true, 5*time.Minute)
	if err != nil {
		return "", errors.Wrap(err, "exec cmd in pod error")
	}
	reg := regexp.MustCompile(`system identifier:[ ]*([\d]+)`)
	params := reg.FindStringSubmatch(stdout)
	if len(params) == 2 {
		return params[1], nil
	}
	return "", errors.New("system identifier is not found.")
}

/**
 * @Description: 创建slot
 * @receiver m
 * @return err
 */
func (m *ManagerClient) CreateReplicationSlot(ctx context.Context, roResourceName string) error {
	envs, err := m.EnvGetStrategy.GetCreateReplicationSlotEnvirons(roResourceName)
	if err != nil {
		return errors.Wrap(err, "GetCreateReplicationSlotEnvirons error")
	}
	return m.ExecCmdInPod(ctx, envs)
}

/**
 * @Description: Install是否执行成功
 * @receiver m
 * @return err
 */
func (m *ManagerClient) CheckInstallIsReady(ctx context.Context) (bool, error) {
	envs, err := m.EnvGetStrategy.GetInstallationProgressEnvirons()
	if err != nil {
		return false, errors.Wrap(err, "GetInstallationProgressEnvirons error")
	}
	stdout, _, err := m.ExecCmdInPodBase(ctx, envs, true, 5*time.Minute)
	if err != nil {
		return false, errors.Wrap(err, "exec cmd in pod error")
	}
	resultBeginIndex := strings.Index(stdout, "\\u0001")
	resultEndIndex := strings.LastIndex(stdout, "\\u0001")
	if resultBeginIndex < 0 || resultEndIndex < 0 {
		return false, errors.New("get installation progress failed")
	}
	resultStr := stdout[resultBeginIndex+len("\\u0001") : resultEndIndex]
	resultStr = strings.ReplaceAll(resultStr, "'", "\"")
	var result = map[string]string{}
	if err = json.Unmarshal([]byte(resultStr), &result); err != nil {
		return false, err
	}
	if result["status"] == "running" {
		return false, nil
	} else if result["status"] == "succeed" {
		return true, nil
	} else if result["status"] == "failed" {
		if errMsg, ok := result["message"]; ok && errMsg == "" {
			return false, errors.New(errMsg)
		}
	}
	return false, errors.New(result["message"])
}

func (m *ManagerClient) loadStrategy(domainModel interface{}, ins *domain.DbIns) error {
	if domainModel == nil {
		return errors.New("domain model is empty")
	}
	if ins == nil {
		return errors.New("instance is empty")
	}
	if m.EnvGetStrategy == nil {
		return errors.New("EnvStrategy is nil")
	}
	if err := m.EnvGetStrategy.Load(domainModel, ins); err != nil {
		return err
	}
	return nil
}

/**
 * @Description: 在Manager容器中执行命令
 * @return error
 */
func (m *ManagerClient) ExecCmdInPod(ctx context.Context, envs string) error {
	_, _, err := m.ExecCmdInPodBase(ctx, envs, true, 5*time.Minute)
	if err != nil {
		return errors.Wrap(err, "exec cmd in pod error")
	}
	return nil
}

// const IgnoreExecErr string = "starting container process caused"
var ContainerNotReadyErrors = []string{
	"starting container process caused",
	"container not found",
	"pod does not exist",
}

func (m *ManagerClient) ExecCmdInPodBase(
	ctx context.Context,
	envs string,
	waitContainer bool,
	waitContainerTimeout time.Duration,
) (stdout string, stderr string, err error) {
	checkNeedRetry := func(stdout, stderr string, errInner error) bool {
		if !waitContainer {
			return false
		}
		for _, errStr := range ContainerNotReadyErrors {
			if strings.Contains(errInner.Error(), errStr) ||
				strings.Contains(stdout, errStr) ||
				strings.Contains(stderr, errStr) {
				return true
			}
		}
		return false
	}

	command := []string{"bash", "-c", envs + " python /docker_script/entry_point.py"}

	execFunc := func(envs string) (stdout string, stderr string, err error) {
		stdout, stderr, err = k8sutil.ExecInPod(command, "manager", m.PodName, m.PodNamespace, m.logger)
		m.logger.V(10).Info(stdout, stderr, err)
		return
	}

	stdout, stderr, err = execFunc(envs)
	if err == nil {
		return
	}
	if !checkNeedRetry(stdout, stderr, err) {
		m.logger.Error(err, "execute cmd failed.", "command", command, "out", stdout)
		return
	}
	retryErr := waitutil.PollImmediateWithContext(ctx, 3*time.Second, waitContainerTimeout, func() (bool, error) {
		if !k8sutil.CheckPodReady(m.PodName, m.PodNamespace, m.logger) {
			return true, fmt.Errorf("[%s %s] pod not ready", m.PodName, m.PodNamespace)
		}
		stdout, stderr, err = execFunc(envs)
		if err == nil {
			return true, nil
		}
		if !checkNeedRetry(stdout, stderr, err) {
			m.logger.Error(err, "execute cmd failed, retry", "command", command, "out", stdout)
			return true, err
		}
		return false, nil
	})
	if retryErr != nil {
		err = errors.Wrap(err, fmt.Sprintf("%v", retryErr))
		m.logger.Error(err, "")
	} else {
		err = nil
	}
	return
}
