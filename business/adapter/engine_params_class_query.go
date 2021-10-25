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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
)

var modeMap = map[domain.EngineType]string{
	domain.EngineTypeDataMax: "DataMax",
	domain.EngineTypeRwo:     "WriteReadMore",
	domain.EngineTypeSingle:  "Single",
	domain.EngineTypeStandby: "WriteReadMore",
}

func NewEngineParamsClassQuery(logger logr.Logger) *EngineParamsClassQuery {
	return &EngineParamsClassQuery{}
}

type EngineParamsClassQuery struct{}

func (q *EngineParamsClassQuery) GetClassParams(engineType domain.EngineType, classKey string) (params map[string]string, err error) {
	labelSet := labels.Set{
		"classKey":      classKey,
		"configtype":    "instance_level",
		"dbClusterMode": modeMap[engineType],
		"dbType":        configuration.GetConfig().DbTypeLabel,
		"dbVersion":     "1.0",
		"leveltype":     "instance_level_config",
	}
	labelSelector := labels.SelectorFromSet(labelSet)
	opts := &client.ListOptions{
		LabelSelector: labelSelector,
	}
	configMapList := &corev1.ConfigMapList{}
	err = mgr.GetSyncClient().List(context.TODO(), configMapList, opts)
	if err != nil && apierrors.IsNotFound(err) {
		return nil, errors.New(fmt.Sprintf("GetClassParams err, %s %s configmap not found: %v", modeMap[engineType], classKey, err))
	} else if err != nil {
		return nil, errors.New(fmt.Sprintf("GetClassParams err: %v", err))
	}
	if len(configMapList.Items) != 1 {
		return nil, errors.New(fmt.Sprintf("GetClassParams err, %s %s configmap items len(%d) not eq 1", modeMap[engineType], classKey, len(configMapList.Items)))
	}
	return configMapList.Items[0].Data, nil
}
