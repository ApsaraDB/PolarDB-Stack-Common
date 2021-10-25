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
	"errors"
	"fmt"
	"strconv"

	"github.com/ApsaraDB/PolarDB-Stack-Common/configuration"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
)

func NewClassQuery(logger logr.Logger) *ClassQuery {
	return &ClassQuery{
		logger: logger.WithValues("component", "ClassQuery"),
	}
}

type ClassQuery struct {
	logger logr.Logger
}

func (q *ClassQuery) GetClasses(engineType domain.EngineType, classKey string) (classList []*domain.EngineClass, err error) {
	labelSet := labels.Set{
		"configtype":    "instance_level",
		"dbClusterMode": modeMap[engineType],
		"dbType":        configuration.GetConfig().DbTypeLabel,
		"dbVersion":     "1.0",
		"leveltype":     "instance_level_resource",
	}
	if classKey != "" {
		labelSet["classKey"] = classKey
	}
	labelSelector := labels.SelectorFromSet(labelSet)
	opts := &client.ListOptions{
		LabelSelector: labelSelector,
	}
	configMapList := &corev1.ConfigMapList{}
	err = mgr.GetSyncClient().List(context.TODO(), configMapList, opts)
	if err != nil && apierrors.IsNotFound(err) {
		return nil, errors.New(fmt.Sprintf("GetClasses err, configmap not found: %v", err))
	} else if err != nil {
		return nil, errors.New(fmt.Sprintf("GetClasses err: %v", err))
	}
	for _, cm := range configMapList.Items {
		classList = append(classList, convertEngineClass(cm))
	}
	return classList, nil
}

func convertEngineClass(cm corev1.ConfigMap) *domain.EngineClass {
	return &domain.EngineClass{
		Name:          cm.Annotations["polarbox.class.name"],
		ClassKey:      cm.Labels["classKey"],
		CpuRequest:    getCpuQuantity(cm.Data, "cpu_cores_request"),
		CpuLimit:      getCpuQuantity(cm.Data, "cpu_cores"),
		MemoryRequest: getMemoryQuantity(cm.Data, "mem_request"),
		MemoryLimit:   getMemoryQuantity(cm.Data, "mem_limit"),
		HugePageLimit: getMemoryQuantity(cm.Data, "hugepage_limit"),
		MemoryShow:    getMemoryQuantity(cm.Data, "mem"),
		StorageMin:    getMemoryQuantity(cm.Data, "storage"),
		StorageMax:    getMemoryQuantity(cm.Data, "max_storage"),
		Resource:      getResource(cm.Data, "class_specified"),
	}
}

func getMemoryQuantity(data map[string]string, field string) *resource.Quantity {
	if strVal, ok := data[field]; !ok {
		val := resource.MustParse("0Mi")
		return &val
	} else if numVal, err := strconv.ParseFloat(strVal, 64); err == nil {
		val := resource.MustParse(fmt.Sprintf("%fMi", numVal))
		return &val
	}
	return nil
}

func getCpuQuantity(data map[string]string, field string) *resource.Quantity {
	if strVal, ok := data[field]; !ok {
		val := resource.MustParse("0m")
		return &val
	} else if cpu, err := strconv.ParseFloat(strVal, 64); err == nil {
		val := resource.MustParse(fmt.Sprintf("%fm", cpu*1000))
		return &val
	}
	return nil
}

func getResource(data map[string]string, field string) map[string]*domain.InstanceResource {
	if classSpecified, ok := data[field]; ok {
		resources := map[string]*domain.InstanceResource{}
		json.Unmarshal([]byte(classSpecified), &resources)
		return resources
	}
	return nil
}
