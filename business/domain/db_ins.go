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

package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/waitutil"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

type DbIns struct {
	DbInsId
	ResourceName      string
	ResourceNamespace string
	Host              string
	HostIP            string
	ClientIP          string
	NetInfo           *NetInfo      // 网络信息
	EngineState       *EngineStatus // 实例状态
	Installed         bool          // 是否已经安装完成
	StorageHostId     string        // pfs每个节点需要一个ID
	TargetNode        string        // 指定调度节点
	ExcludedNodes     []string      // 排除的节点
	inited            bool

	ManagerClient IManagerClient
	PodManager    IEnginePodManager
}

type DbInsId struct {
	PhysicalInsId string // 物理ID
	InsId         string // 实例ID
	InsName       string // 等于InsId的初始值，后续重建实例该值不会变
}

type NetInfo struct {
	FloatingIP      string
	Port            int
	ReplicationIP   string
	ReplicationPort int
}

type EngineState struct {
	StartedAt *time.Time
	Reason    string
	State     string
}

type EngineStatus struct {
	CurrentState EngineState
	LastState    EngineState
}

type InstanceResource struct {
	CPUCores    resource.Quantity `json:"cpu_cores"`
	LimitMemory resource.Quantity `json:"limit_memory"`
	Config      string            `json:"config"`
}

var insNotInitError = errors.New("instance not init.")

func (ins *DbIns) Init(managerClient IManagerClient, podManager IEnginePodManager) {
	ins.ManagerClient = managerClient
	ins.PodManager = podManager
	ins.inited = true
}

/**
 * @Description: 创建Pod
 * @receiver ins
 * @return error
 */
func (ins *DbIns) CreatePod(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.PodManager.SetIns(ins)
	if ins.Installed {
		return nil
	}
	return ins.PodManager.CreatePodAndWaitReady(ctx)
}

/**
 * @Description: 删除Pod
 * @receiver ins
 * @return error
 */
func (ins *DbIns) DeletePod(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.PodManager.SetIns(ins)
	return ins.PodManager.DeletePod(ctx)
}

/**
 * @Description: 先停止引擎运行再删除Pod
 * @receiver ins
 * @return error
 */
func (ins *DbIns) StopEngineAndDeletePod(ctx context.Context, ignoreStopError bool) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.PodManager.SetIns(ins)
	ins.ManagerClient.SetIns(ins)

	if isDeleted, err := ins.PodManager.IsDeleted(ctx); err != nil {
		return err
	} else if isDeleted {
		return nil
	}

	if err := ins.ManagerClient.GracefulStopEngine(ctx); err != nil && !ignoreStopError {
		return err
	}

	return ins.PodManager.DeletePod(ctx)
}

func (ins *DbIns) EnsureInsTypeMeta(ctx context.Context, insType string) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.PodManager.SetIns(ins)
	return ins.PodManager.EnsureInsTypeMeta(ctx, insType)
}

/**
 * @Description: 安装DB引擎
 * @receiver ins
 * @return error
 */
func (ins *DbIns) InstallDbIns(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	if ins.Installed {
		return nil
	}
	return ins.ManagerClient.InstallEngine(ctx)
}

/**
 * @Description: 安装DB引擎
 * @receiver ins
 * @return error
 */
func (ins *DbIns) GetPrimarySystemIdentifier(ctx context.Context) (string, error) {
	if !ins.inited {
		return "", insNotInitError
	}
	ins.ManagerClient.SetIns(ins)

	return ins.ManagerClient.GetPrimarySystemIdentifier(ctx)
}

/**
 * @Description: 创建UE所需元数据
 * @receiver ins
 * @return error
 */
func (ins *DbIns) SetupLogAgent(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	if ins.Installed {
		return nil
	}
	return ins.ManagerClient.SetupLogAgent(ctx)
}

/**
 * @Description: 记录引擎安装完毕
 * @receiver ins
 * @return error
 */
func (ins *DbIns) SetDbInsInstalled() error {
	ins.Installed = true
	return nil
}

/**
 * @Description: 根据账户元数据信息创建账户
 * @receiver ins
 * @return error
 */
func (ins *DbIns) CreateAccount(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	return ins.ManagerClient.CreateEngineAccount(ctx)
}

/**
 * @Description: 健康检查
 * @receiver ins
 * @return error
 */
func (ins *DbIns) DoHealthCheck(ctx context.Context) error {
	return ins.DoHealthCheckWithTimeout(ctx, 5*time.Minute)
}

/**
 * @Description: 等待复制初始数据完成
 * @receiver ins
 * @return error
 */
func (ins *DbIns) WaitForInstallReady(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	return ins.checkInstallProgressWithTimeout(ctx, 2*time.Minute)
}

/**
 * @Description: 创建复制槽
 * @receiver ins
 * @return error
 */
func (ins *DbIns) CreateReplicationSlot(ctx context.Context, roResourceName string) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	return ins.ManagerClient.CreateReplicationSlot(ctx, roResourceName)
}

/**
 * @Description: 健康检查
 * @receiver ins
 * @return error
 */
func (ins *DbIns) DoHealthCheckWithTimeout(ctx context.Context, timeout time.Duration) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	return ins.checkHealthWithTimeout(ctx, timeout)
}

/**
 * @Description: 执行刷参
 * @receiver ins
 * @return error
 */
func (ins *DbIns) FlushParams(ctx context.Context, paramItems map[string]string, restart bool) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	if err := ins.DoHealthCheckWithTimeout(ctx, time.Minute); err != nil {
		// 如果等了1分钟还处于非健康状态，可能是上次执行到stop了未启动，先尝试启动，再等待健康
		if err := ins.ManagerClient.StartEngine(ctx); err != nil {
			return err
		}
		if err := ins.DoHealthCheck(ctx); err != nil {
			return err
		}
	}
	if err := ins.ManagerClient.UpdateEngineParams(ctx, paramItems); err != nil {
		return err
	}
	if restart {
		return ins.ManagerClient.RestartEngine(ctx)
	}
	return nil
}

/**
 * @Description: 重启实例
 * @receiver ins
 * @return error
 */
func (ins *DbIns) Restart(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	if err := ins.ManagerClient.RestartEngine(ctx); err != nil {
		return err
	}
	return ins.DoHealthCheck(ctx)
}

/**
 * @Description: 安全停止引擎
 * @return error
 */
func (ins *DbIns) GracefulStopEngine(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	return ins.ManagerClient.GracefulStopEngine(ctx)
}

/**
 * @Description: 启动引擎
 * @return error
 */
func (ins *DbIns) StartEngine(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	return ins.ManagerClient.StartEngine(ctx)
}

/**
 * @Description: 存储扩容
 * @return error
 */
func (ins *DbIns) GrowStorage(ctx context.Context) error {
	if !ins.inited {
		return insNotInitError
	}
	ins.ManagerClient.SetIns(ins)
	return ins.ManagerClient.GrowStorage(ctx)
}

func (ins *DbIns) checkHealthWithTimeout(ctx context.Context, timeout time.Duration) error {
	var (
		err              error
		latestErr        error
		timeoutBeginTime = time.Now()
		timeoutErr       = errors.New(fmt.Sprintf("timeout waiting for engine to be ready for %v", timeout))
	)
	err = waitutil.PollInfinite(ctx, 5*time.Second, func() (bool, error) {
		if timeoutBeginTime.Add(timeout).Before(time.Now()) {
			return false, timeoutErr
		}
		checkErr := ins.ManagerClient.CheckHealth(ctx)
		if checkErr == nil {
			return true, nil
		}
		latestErr = checkErr.Error
		if checkErr.Name == HealthErrorStartingUp {
			// starting up 一直重试，不管过期时间
			timeoutBeginTime = time.Now()
		} else if checkErr.Name == HealthErrorContainerNotFound {
			// 容器没有找到，可能创建完还没就绪，重试知道过期
		}
		// 其它错误，重试
		return false, nil
	})

	if err != nil {
		if err == timeoutErr {
			err = errors.Wrap(err, fmt.Sprintf("latest error: %v", latestErr))
		}
		return errors.Wrap(err, fmt.Sprintf("error occurred when waiting for engine health"))
	}

	return nil
}

func (ins *DbIns) checkInstallProgressWithTimeout(ctx context.Context, timeout time.Duration) error {
	var (
		err              error
		latestErr        error
		timeoutBeginTime = time.Now()
		timeoutErr       = errors.New(fmt.Sprintf("timeout waiting for install to be ready for %v", timeout))
	)
	err = waitutil.PollInfinite(ctx, 5*time.Second, func() (bool, error) {
		if timeoutBeginTime.Add(timeout).Before(time.Now()) {
			return false, timeoutErr
		}
		ready, checkErr := ins.ManagerClient.CheckInstallIsReady(ctx)
		if ready {
			return true, nil
		}
		latestErr = checkErr
		if !ready && checkErr == nil {
			// 一直重试，不管过期时间
			timeoutBeginTime = time.Now()
		}
		// 其它错误，重试
		return false, nil
	})

	if err != nil {
		if err == timeoutErr {
			err = errors.Wrap(err, fmt.Sprintf("latest error: %v", latestErr))
		}
		return errors.Wrap(err, fmt.Sprintf("error occurred when waiting for install ready"))
	}

	return nil
}
