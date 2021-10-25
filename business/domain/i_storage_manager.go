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

type StorageTopo struct {
	ReadNodes []*StorageTopoNode `json:"read_nodes"`
	WriteNode *StorageTopoNode   `json:"write_node"`
}

type StorageTopoNode struct {
	NodeId string `json:"node_id"`
	NodeIP string `json:"node_ip"`
}

type IStorageManager interface {
	/**
	 * @Description: 初始化
	 * @param ip
	 * @param port
	 * @return error
	 */
	InitWithAddress(ip, port string) error

	/**
	 * @Description: 使用存储
	 * @param ctx
	 * @param name pvc名称
	 * @param name pvc namespace
	 * @param volumeId
	 * @param resourceName 关联资源名称
	 * @param volumeType
	 * @param format 是否格式化
	 * @return error
	 */
	UseStorage(ctx context.Context, name, namespace, volumeId, resourceName string, volumeType string, format bool) error

	/**
	 * @Description: 设置写锁
	 * @param ctx
	 * @param name pvc名称
	 * @param name pvc namespace
	 * @param nodeId 写节点ID
	 * @return *StorageTopo 设置后的读写节点拓扑关系
	 * @return error
	 */
	SetWriteLock(ctx context.Context, name, namespace, nodeId string) (*StorageTopo, error)

	/**
	 * @Description: 获取读写节点拓扑关系
	 * @param ctx
	 * @param name pvc名称
	 * @param name pvc namespace
	 * @return *StorageTopo 最新的读写节点拓扑关系
	 * @return error
	 */
	GetTopo(ctx context.Context, name, namespace string) (*StorageTopo, error)

	/**
	 * @Description: 扩容
	 * @param ctx
	 * @param name pvc名称
	 * @param name pvc namespace
	 * @param volumeId
	 * @param resourceName 关联资源名称
	 * @param volumeType
	 * @param reqSize 扩容后容量
	 * @return error
	 */
	Expand(ctx context.Context, name, namespace, volumeId, resourceName string, volumeType int, reqSize int64) error

	/**
	 * @Description: 释放存储
	 * @param ctx
	 * @param name pvc名称
	 * @param name pvc namespace
	 * @param cluster 防止误释放其它集群使用的存储；为空则不校验存储是否被该集群使用，直接释放
	 * @return error
	 */
	Release(ctx context.Context, name, namespace, clusterName string) error
}
