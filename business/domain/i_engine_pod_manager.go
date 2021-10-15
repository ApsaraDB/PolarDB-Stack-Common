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

type OnPfsdContainerReady func() error

/**
 * @Description: DataMax容器管理
 */
type IEnginePodManager interface {

	/**
	 * @Description: 初始化
	 * @param domainModel datamax领域模型
	 */
	Init(domainModel interface{})

	/**
	 * @Description: 设置对应的实例
	 * @param ins 因为一个DB集群会可能包含多个实例，这里指定操作哪个实例
	 */
	SetIns(ins *DbIns)

	/**
	 * @Description: 创建Pod并等待Pod就绪
	 * @param resourceName pod名称
	 * @param ctx
	 * @return error
	 */
	CreatePodAndWaitReady(ctx context.Context) error

	/**
	 * @Description: 删除Pod
	 * @param resourceName pod名称
	 * @param ctx
	 * @return error
	 */
	DeletePod(ctx context.Context) error

	/**
	 * @Description: pod是否已经被删除
	 * @param ctx
	 * @return error
	 */
	IsDeleted(ctx context.Context) (bool, error)

	/**
	 * @Description: 确保pod的读写类型正确
	 * @param ctx
	 * @param insType
	 * @return error
	 */
	EnsureInsTypeMeta(ctx context.Context, insType string) error
}
