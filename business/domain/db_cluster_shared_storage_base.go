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
	"encoding/json"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
)

// 基于共享存储的DB集群基类
type SharedStorageDbClusterBase struct {
	DbClusterBase
	StorageInfo    *StorageInfo
	StorageManager IStorageManager
	inited         bool
}

type StorageInfo struct {
	DiskID     string
	DiskQuota  string
	VolumeId   string
	VolumeType string
}

var clusterNotInitError = errors.New("cluster not init.")

func (dbCluster *SharedStorageDbClusterBase) Init(
	paramsTemplateQuery IEngineParamsTemplateQuery,
	paramsClassQuery IEngineParamsClassQuery,
	paramsRepo IEngineParamsRepository,
	minorVersionQuery IMinorVersionQuery,
	accountRepo IAccountRepository,
	idGenerator IIdGenerator,
	portGenerator IPortGenerator,
	storageManager IStorageManager,
	classQuery IClassQuery,
	clusterManagerClient IClusterManagerClient,
	hostClusterQuery IHostClusterQuery,
	clusterManagerRemover IClusterManagerRemover,
	logger logr.Logger,
) {
	dbCluster.StorageManager = storageManager
	dbCluster.DbClusterBase.Init(
		paramsTemplateQuery,
		paramsClassQuery,
		paramsRepo,
		minorVersionQuery,
		accountRepo,
		idGenerator,
		portGenerator,
		classQuery,
		clusterManagerClient,
		hostClusterQuery,
		clusterManagerRemover,
		logger,
	)
	dbCluster.inited = true
}

/**
 * @Description: 使用存储
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *SharedStorageDbClusterBase) UseStorage(ctx context.Context, format bool) error {
	if !dbCluster.inited {
		return clusterNotInitError
	}
	return dbCluster.StorageManager.UseStorage(ctx, dbCluster.StorageInfo.DiskID, dbCluster.Namespace, dbCluster.StorageInfo.VolumeId, dbCluster.Name, dbCluster.StorageInfo.VolumeType, format)
}

/**
 * @Description: 为存储设置写锁
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *SharedStorageDbClusterBase) SetStorageWriteLock(ctx context.Context, nodeId string) error {
	if !dbCluster.inited {
		return clusterNotInitError
	}
	topo, err := dbCluster.StorageManager.SetWriteLock(ctx, dbCluster.StorageInfo.DiskID, dbCluster.Namespace, nodeId)
	if err != nil {
		if topo != nil {
			topoStr, _ := json.Marshal(topo)
			return errors.Wrapf(err, "current topo: [%s]", topoStr)
		}
		return err
	}
	return nil
}

/**
 * @Description: 释放存储
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *SharedStorageDbClusterBase) ReleaseStorage(ctx context.Context) error {
	if !dbCluster.inited {
		return clusterNotInitError
	}

	if err := dbCluster.StorageManager.Release(ctx, dbCluster.StorageInfo.DiskID, dbCluster.Namespace, dbCluster.Name); err != nil {
		if strings.Contains(err.Error(), "used by other cluster") {
			dbCluster.Logger.Info(err.Error() + ", ignore.")
			return nil
		}
		return err
	}
	return nil
}
