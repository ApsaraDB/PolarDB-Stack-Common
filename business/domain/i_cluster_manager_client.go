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
)

type CopyType string

const (
	CopyTypeSync  CopyType = "sync"
	CopyTypeAsync CopyType = "async"
)

type ClusterStatus struct {
	Phase   string      `json:"phase"`
	Rw      InsStatus   `json:"rw"`
	Ro      []InsStatus `json:"ro"`
	Standby []InsStatus `json:"standby,omitempty"`
}

type InsStatus struct {
	Endpoint   string `json:"endpoint"`
	PodName    string `json:"pod_name"`
	Phase      string `json:"phase"`
	StartAt    string `json:"start_at"`
	SyncStatus string `json:"sync_status,omitempty"`
	Role       string `json:"role,omitempty"`
}

/**
 * @Description: Cluster Manager Client
 */
type IClusterManagerClient interface {

	/**
	 * @Description: 设置跨域主集群
	 * @param ctx
	 * @return error
	 */
	SetLeader(ctx context.Context) error

	/**
	 * @Description: 取消跨域主集群
	 * @param ctx
	 * @return error
	 */
	SetFollower(ctx context.Context) error

	/**
	 * @Description: 初始化
	 * @param ctx
	 * @param dbClusterNamespace
	 * @param dbClusterName
	 * @param waitCmReady
	 * @return error
	 */
	InitWithLocalDbCluster(ctx context.Context, dbClusterNamespace, dbClusterName string, waitCmReady bool) error

	/**
	 * @Description: 添加DataMax节点
	 * @param ctx
	 * @param id
	 * @param ip
	 * @param podName
	 * @param port
	 * @return error
	 */
	AddDataMax(ctx context.Context, id, ip, podName string, port int) error

	/**
	 * @Description: 更新DataMax节点
	 * @param ctx
	 * @param id
	 * @param ip
	 * @param podName
	 * @param port
	 * @return error
	 */
	UpdateDataMax(ctx context.Context, id, ip, podName string, port int) error

	/**
	 * @Description: 删除DataMax节点
	 * @param ctx
	 * @param id
	 * @return error
	 */
	DeleteDataMax(ctx context.Context, id string) error

	/**
	 * @Description: 添加集群节点
	 * @param ctx
	 * @param id
	 * @param ip
	 * @param port
	 * @param isClientIP
	 * @return error
	 */
	AddCluster(ctx context.Context, id, ip string, port int, isClientIP bool) error

	/**
	 * @Description: 更新集群节点
	 * @param ctx
	 * @param id
	 * @param ip
	 * @param port
	 * @param isClientIP
	 * @return error
	 */
	UpdateCluster(ctx context.Context, id, ip string, port int, isClientIP bool) error

	/**
	 * @Description: 删除集群节点
	 * @param ctx
	 * @param id
	 * @return error
	 */
	DeleteCluster(ctx context.Context, id string) error

	/**
	 * @Description: 创建拓扑关系
	 * @param ctx
	 * @param upStreamId 上游节点ID
	 * @param downStreamId 下游节点ID
	 * @param copyType 复制关系类型
	 * @return error
	 */
	AddTopologyEdge(ctx context.Context, upStreamId, downStreamId string, copyType CopyType) error

	/**
	 * @Description: 删除拓扑关系
	 * @param ctx
	 * @param upStreamId 上游节点ID
	 * @param downStreamId 下游节点ID
	 * @return error
	 */
	DeleteTopologyEdge(ctx context.Context, upStreamId, downStreamId string) error

	/**
	 * @Description: 设置standby订阅DataMax
	 * @param ctx
	 * @param id 集群ID
	 * @param ip 上游 client IP
	 * @param port 上游 端口
	 * @param copyType 复制关系类型
	 * @param upStreamId 上游ID
	 * @return error
	 */
	DemoteStandby(ctx context.Context, id, ip string, port int, copyType CopyType, upStreamId string) error

	/**
	 * @Description: standby promote
	 * @param ctx
	 * @return error
	 */
	PromoteStandby(ctx context.Context) error

	/**
	 * @Description: 禁用HA
	 * @param ctx
	 * @return error
	 */
	DisableHA(ctx context.Context) error

	/**
	 * @Description: 启用HA
	 * @param ctx
	 * @return error
	 */
	EnableHA(ctx context.Context) error

	/**
	 * @Description: 添加实例
	 * @param ctx
	 * @param id
	 * @param ip
	 * @param port
	 * @return error
	 */
	AddIns(ctx context.Context, id, ip, role, sync, hostName string, port int, isClientIP bool) error

	/**
	 * @Description: 删除实例
	 * @param ctx
	 * @param id
	 * @param ip
	 * @param port
	 * @return error
	 */
	RemoveIns(ctx context.Context, id, ip, role, sync, hostName string, port int, isClientIP bool) error

	/**
	 * @Description: 读写切换
	 * @param ctx
	 * @param oriRwIp
	 * @param oriRoIp
	 * @param port
	 * @return error
	 */
	Switchover(ctx context.Context, oriRwIp, oriRoIp, port string, isClientIP bool) error

	/**
	 * @Description: 获取集群状态
	 * @param ctx
	 * @return error
	 */
	GetClusterStatus(ctx context.Context) (*ClusterStatus, error)

	EnsureAffinity(ctx context.Context, clusterNamespace, clusterName, rwHostName string) error

	UpgradeVersion(ctx context.Context, clusterNamespace, clusterName, image string) error
}
