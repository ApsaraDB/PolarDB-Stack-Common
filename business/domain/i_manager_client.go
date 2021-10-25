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

import "context"

type HealthCheckError struct {
	Name   HealthErrorEnum
	Error  error  // 错误
	Reason string // 出错原因
}

func CreateHealthError(name HealthErrorEnum, reason string, err error) *HealthCheckError {
	return &HealthCheckError{
		Name:   name,
		Reason: reason,
		Error:  err,
	}
}

type HealthErrorEnum string

const (
	HealthErrorStartingUp        HealthErrorEnum = "StartingUp"
	HealthErrorContainerNotFound HealthErrorEnum = "ContainerNotFound"
	HealthErrorUnKnown           HealthErrorEnum = "Unknown"
)

/**
 * @Description: Manager客户端
 */
type IManagerClient interface {

	/**
	 * @Description: 初始化ManagerClient
	 * @param ctx
	 * @param domainModel 资源对应的领域模型
	 */
	Init(domainModel interface{})

	/**
	 * @Description: 设置ManagerClient对应的实例
	 * @param ins 因为一个DB集群会可能包含多个实例，这里指定操作哪个实例
	 * @return error
	 */
	SetIns(ins *DbIns) error

	/**
	 * @Description: 安装引擎
	 * @param ctx
	 * @return error
	 */
	InstallEngine(ctx context.Context) error

	/**
	 * @Description: 创建UE所需元数据
	 * @param ctx
	 * @return error
	 */
	SetupLogAgent(ctx context.Context) error

	/**
	 * @Description: 启动引擎
	 * @param ctx
	 * @return error
	 */
	StartEngine(ctx context.Context) error

	/**
	 * @Description: 安全停止引擎
	 * @param ctx
	 * @return error
	 */
	GracefulStopEngine(ctx context.Context) error

	/**
	 * @Description: 重启引擎
	 * @param ctx
	 * @return error
	 */
	RestartEngine(ctx context.Context) error

	/**
	 * @Description: 锁定引擎
	 * @param ctx
	 * @return error
	 */
	LockEngine(ctx context.Context) error

	/**
	 * @Description: 创建账户
	 * @param ctx
	 * @return error
	 */
	CreateEngineAccount(ctx context.Context) error

	/**
	 * @Description: 更新引擎参数
	 * @param ctx
	 * @return error
	 */
	UpdateEngineParams(ctx context.Context, engineParams map[string]string) error

	/**
	 * @Description: 存储扩容
	 * @param ctx
	 * @return error
	 */
	GrowStorage(ctx context.Context) error

	/**
	 * @Description: 健康检查
	 * @param ctx
	 * @return error
	 */
	CheckHealth(ctx context.Context) (err *HealthCheckError)

	/**
	 * @Description: 强制停止引擎
	 * @param ctx
	 * @return error
	 */
	ForceStopEngine(ctx context.Context) error

	/**
	 * @Description: 获取PrimarySystemIdentifier
	 * @param ctx
	 * @return error
	 */
	GetPrimarySystemIdentifier(ctx context.Context) (string, error)

	/**
	 * @Description: 创建slot
	 * @param ctx
	 * @return error
	 */
	CreateReplicationSlot(ctx context.Context, roResourceName string) error

	/**
	 * @Description: 异步install是否执行成功
	 * @receiver m
	 * @return err
	 */
	CheckInstallIsReady(ctx context.Context) (bool, error)
}
