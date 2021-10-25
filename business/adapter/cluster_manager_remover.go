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
	"fmt"

	standbydefine "github.com/ApsaraDB/PolarDB-Stack-Common/define"
	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
)

func NewClusterManagerRemover(logger logr.Logger) *ClusterManagerRemover {
	return &ClusterManagerRemover{
		logger: logger.WithValues("component", "ClusterManagerRemover"),
	}
}

type ClusterManagerRemover struct {
	logger logr.Logger
}

func (m *ClusterManagerRemover) Remove(dbClusterName, dbClusterNamespace string, ctx context.Context) error {
	clusterManagerName := dbClusterName + standbydefine.ClusterManagerNameSuffix
	clusterManager := &appsv1.Deployment{}
	err := mgr.GetSyncClient().Get(context.TODO(), types.NamespacedName{
		Name:      clusterManagerName,
		Namespace: dbClusterNamespace,
	}, clusterManager)
	if err != nil && !apierrors.IsNotFound(err) {
		m.logger.Error(err, fmt.Sprintf("get cm %s deployment error", clusterManagerName))
		return err
	}

	if err == nil && *clusterManager.Spec.Replicas > 0 {
		// cm deployment 存在
		// 先禁用cm，否则流程中它会触发切换，导致前后步骤元数据不一致
		// disable是同步操作，所有切换完成后他才会返回结果，后续步骤都会使用最新的元数据
		cmClient := NewClusterManagerClient(m.logger)
		if err := cmClient.InitWithLocalDbCluster(ctx, dbClusterNamespace, dbClusterName, true); err != nil {
			return err
		}
		if err = cmClient.DisableHA(ctx); err != nil {
			m.logger.Error(err, "DisableHA error")
			return err
		}

		err = mgr.GetSyncClient().Delete(context.TODO(), clusterManager, client.GracePeriodSeconds(0))
		if err != nil {
			m.logger.Error(err, fmt.Sprintf("delete cm %s deployment error", clusterManagerName))
			return err
		}
	}

	if err := waitForCmDeleted(dbClusterName, dbClusterNamespace, m.logger, ctx); err != nil {
		m.logger.Error(err, fmt.Sprintf("%s waitForCmDeleted error", clusterManagerName))
		return err
	}
	configmapNames := []string{
		dbClusterName + standbydefine.ClusterManagerNameSuffix,
		dbClusterName + standbydefine.ClusterManagerMetaSuffix,
		dbClusterName + standbydefine.ClusterManagerFlagSuffix,
		dbClusterName + standbydefine.ClusterManagerLeaderSuffix,
		dbClusterName + standbydefine.ClusterManagerGlobalConfigSuffix,
	}
	for _, configmapName := range configmapNames {
		configmap := &corev1.ConfigMap{}
		err := mgr.GetSyncClient().Get(context.TODO(), types.NamespacedName{
			Name:      configmapName,
			Namespace: dbClusterNamespace,
		}, configmap)
		if err != nil && !apierrors.IsNotFound(err) {
			m.logger.Error(err, fmt.Sprintf("get cm %s configmap %s error", clusterManagerName, configmapName))
			return err
		}
		if err == nil {
			err = mgr.GetSyncClient().Delete(context.TODO(), configmap, client.GracePeriodSeconds(0))
			m.logger.Info(fmt.Sprintf("delete cm %s configmap %s", clusterManagerName, configmapName))
			if err != nil {
				m.logger.Error(err, fmt.Sprintf("delete cm %s configmap %s error", clusterManagerName, configmapName))
				return err
			}
		}
	}
	return nil
}
