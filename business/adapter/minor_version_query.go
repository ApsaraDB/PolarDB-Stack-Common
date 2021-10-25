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
	"errors"
	"fmt"

	"github.com/ApsaraDB/PolarDB-Stack-Common/configuration"

	"github.com/go-logr/logr"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils"

	"github.com/ApsaraDB/PolarDB-Stack-Common/define"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
)

var modeMinorVersionMap = map[domain.EngineType]string{
	domain.EngineTypeDataMax: "WriteReadMore", // datamax和dbcluster使用相同的镜像及版本
	domain.EngineTypeRwo:     "WriteReadMore",
	domain.EngineTypeSingle:  "Single",
	domain.EngineTypeStandby: "WriteReadMore", // standby和dbcluster使用相同的镜像及版本
}

func NewMinorVersionQuery(logger logr.Logger) *MinorVersionQuery {
	return &MinorVersionQuery{
		logger: logger.WithValues("component", "MinorVersionQuery"),
	}
}

type MinorVersionQuery struct {
	logger logr.Logger
}

func (q *MinorVersionQuery) GetMinorVersions(engineType domain.EngineType) (latestVersion string, versionList []*domain.MinorVersion, err error) {
	configMapList, err := getMinorVersionCmByLabels(engineType)
	if err != nil {
		return "", nil, err
	}
	latestVersion = ""
	for _, cm := range configMapList.Items {
		if cm.Data["latestVersion"] != "" {
			latestVersion = cm.Data["latestVersion"]
			continue
		}
		versionList = append(versionList, convertMinorVersion(cm))
	}
	return latestVersion, versionList, nil
}

func (q *MinorVersionQuery) GetLatestMinorVersion(engineType domain.EngineType) (version *domain.MinorVersion, err error) {
	cm, err := k8sutil.GetConfigMap(fmt.Sprintf("%s%s-root", configuration.GetConfig().MinorVersionCmName, classSuffix[engineType]), define.MinorVersionCmNamespace, q.logger)
	if err != nil {
		return nil, err
	}
	versionName, ok := cm.Data["latestVersion"]
	if !ok || versionName == "" {
		return nil, errors.New("latest version is empty")
	}
	return q.GetMinorVersion(engineType, versionName)
}

func (q *MinorVersionQuery) GetMinorVersion(engineType domain.EngineType, versionName string) (version *domain.MinorVersion, err error) {
	cmName := utils.FormatString(fmt.Sprintf("%s%s-%s", configuration.GetConfig().MinorVersionCmName, classSuffix[engineType], versionName))
	cm, err := k8sutil.GetConfigMap(cmName, define.MinorVersionCmNamespace, q.logger)
	if err != nil {
		return nil, err
	}
	return convertMinorVersion(*cm), nil
}

func getMinorVersionCmByLabels(engineType domain.EngineType) (*corev1.ConfigMapList, error) {
	labelSet := labels.Set{
		"configtype":    "minor_version_info",
		"dbClusterMode": modeMinorVersionMap[engineType],
		"dbType":        configuration.GetConfig().DbTypeLabel,
		"dbVersion":     "1.0",
	}
	labelSelector := labels.SelectorFromSet(labelSet)
	opts := &client.ListOptions{
		LabelSelector: labelSelector,
	}
	configMapList := &corev1.ConfigMapList{}
	err := mgr.GetSyncClient().List(context.TODO(), configMapList, opts)
	if err != nil && apierrors.IsNotFound(err) {
		return nil, errors.New(fmt.Sprintf("GetMinorVersions err, configmap not found: %v", err))
	} else if err != nil {
		return nil, errors.New(fmt.Sprintf("GetMinorVersions err: %v", err))
	}
	return configMapList, nil
}

func convertMinorVersion(cm corev1.ConfigMap) *domain.MinorVersion {
	result := &domain.MinorVersion{
		Name:   cm.Data["name"],
		Images: make(map[string]string),
	}
	for key, value := range cm.Data {
		if key != "name" {
			imageKey := key
			if key == "pgEngineImage" {
				imageKey = "engineImage"
			} else if key == "pgManagerImage" {
				imageKey = "managerImage"
			} else if key == "pgClusterManagerImage" {
				imageKey = "clusterManagerImage"
			}
			result.Images[imageKey] = value
		}
	}
	return result
}
