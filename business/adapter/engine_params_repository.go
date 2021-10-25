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
	"strings"
	"time"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	"github.com/ApsaraDB/PolarDB-Stack-Common/define"
	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type GetKubeResourceFunc func(name, namespace string, clusterType domain.DbClusterType) (metav1.Object, error)

type EngineParamsRepository struct {
	Logger              logr.Logger
	GetKubeResourceFunc GetKubeResourceFunc
}

/**
 * @Description: 保存用户设置的参数
 * @param engine
 * @param initParams
 * @return error
 */
func (repo *EngineParamsRepository) SaveUserParams(engine *domain.DbClusterBase, initParams map[string]string) error {
	return createParamsConfigMap(engine, initParams, define.UserParamsSuffix, repo.Logger.WithName(engine.Namespace).WithName(engine.Name), repo.GetKubeResourceFunc)
}

/**
 * @Description: 获取用户设置的参数
 * @param engine
 * @return map[string]string
 * @return string 最后一次成功刷参的时间
 * @return error
 */
func (repo *EngineParamsRepository) GetUserParams(engine *domain.DbClusterBase) (map[string]*domain.ParamItem, string, error) {
	items, cm, err := getParamsConfigMapValues(engine.Name, engine.Namespace, define.UserParamsSuffix, repo.Logger.WithName(engine.Namespace).WithName(engine.Name))
	if err != nil {
		return nil, "", err
	}
	var latestFlushTime string
	if cm.Annotations != nil {
		latestFlushTime = cm.Annotations["latestFlushTime"]
	}
	return items, latestFlushTime, nil
}

/**
 * @Description: 保存生效参数，初始化时使用，后面由cm更新
 * @param engine
 * @param initParams
 * @return error
 */
func (repo *EngineParamsRepository) SaveRunningParams(engine *domain.DbClusterBase, initParams map[string]string) error {
	return createParamsConfigMap(engine, initParams, define.RunningParamsSuffix, repo.Logger.WithName(engine.Namespace).WithName(engine.Name), repo.GetKubeResourceFunc)
}

/**
 * @Description: 获取运行中的参数
 * @param engine
 * @return map[string]string
 * @return error
 */
func (repo *EngineParamsRepository) GetRunningParams(engine *domain.DbClusterBase) (map[string]*domain.ParamItem, error) {
	items, _, err := getParamsConfigMapValues(engine.Name, engine.Namespace, define.RunningParamsSuffix, repo.Logger.WithName(engine.Namespace).WithName(engine.Name))
	return items, err
}

/**
 * @Description: 保存最后刷参成功时间
 * @param dbClusterName
 * @return map[string]string
 * @return error
 */
func (repo *EngineParamsRepository) SaveLatestFlushTime(engine *domain.DbClusterBase) error {
	cm, err := k8sutil.GetConfigMap(engine.Name+define.UserParamsSuffix, engine.Namespace, repo.Logger.WithName(engine.Namespace).WithName(engine.Name))
	if err != nil {
		return err
	}

	if cm.Annotations == nil {
		cm.Annotations = map[string]string{}
	}
	cm.Annotations["latestFlushTime"] = time.Now().Format("2006-01-02T15:04:05Z")
	return k8sutil.UpdateConfigMap(cm)
}

/**
 * @Description: 更新用户设置的参数
 * @param engine
 * @param initParams
 * @return error
 */
func (repo *EngineParamsRepository) UpdateUserParams(engine *domain.DbClusterBase, userParams map[string]*domain.EffectiveUserParamValue) error {
	return updateUserParams(engine, userParams, repo.Logger.WithName(engine.Namespace).WithName(engine.Name))
}

func updateUserParams(engine *domain.DbClusterBase, userParams map[string]*domain.EffectiveUserParamValue, logger logr.Logger) error {
	cm, err := k8sutil.GetConfigMap(engine.Name+define.UserParamsSuffix, engine.Namespace, logger)
	if err != nil {
		return err
	}
	changed := false
	for key, value := range cm.Data {
		if userParamValue, ok := userParams[key]; ok {
			entity := &domain.ParamItem{}
			err = json.Unmarshal([]byte(value), entity)
			if err != nil {
				logger.Error(err, "updateUserParams json.Unmarshal error", "value", value)
				return err
			}
			if userParamValue.Value != entity.Value {
				entity.Value = userParamValue.Value
				if userParamValue.NeedFlush {
					entity.UpdateTime = time.Now().Format("2006-01-02T15:04:05Z")
				} else {
					entity.UpdateTime = userParamValue.RunningUpdateTime
				}
				bytes, err := json.Marshal(entity)
				if err != nil {
					logger.Error(err, "updateUserParams json.Marshal error", "entity", entity)
					return err
				}
				cm.Data[key] = string(bytes)
				changed = true
			}
		} else {
			delete(cm.Data, key)
			changed = true
		}
	}
	if changed {
		if err := k8sutil.UpdateConfigMap(cm); err != nil {
			logger.Error(err, "updateUserParams error")
			return err
		}
	}

	return nil
}

func getParamsConfigMapValues(resourceName, resourceNamespace, suffix string, logger logr.Logger) (map[string]*domain.ParamItem, *corev1.ConfigMap, error) {
	var result = make(map[string]*domain.ParamItem)
	cm, err := k8sutil.GetConfigMap(resourceName+suffix, resourceNamespace, logger)
	if err != nil {
		return nil, nil, err
	}
	for key, value := range cm.Data {
		entity := &domain.ParamItem{}
		err = json.Unmarshal([]byte(value), entity)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "getParamsConfigMapValues Unmarshal [%s] error", value)
		}
		result[key] = entity
	}
	return result, cm, nil
}

func createParamsConfigMap(engine *domain.DbClusterBase, paramsMap map[string]string, suffix string, logger logr.Logger, getKubeResourceFunc GetKubeResourceFunc) error {
	name := engine.Name + suffix
	namespace := engine.Namespace
	cm, err := k8sutil.GetConfigMap(name, namespace, logger)
	if err == nil && cm != nil {
		err := k8sutil.DeleteConfigmap(cm, logger)
		if err != nil {
			return err
		}
	}
	entityMap, err := convertParams(paramsMap)
	if err != nil {
		return err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"dbCluster":  name,
				"configType": strings.TrimLeft(suffix, "-"),
			},
		},
		Data: entityMap,
	}
	resource, err := getKubeResourceFunc(engine.Name, namespace, engine.DbClusterType)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(resource, configMap, mgr.GetManager().GetScheme()); err != nil {
		return err
	}

	err = mgr.GetSyncClient().Create(context.TODO(), configMap)
	if err != nil {
		return err
	}
	return nil
}

// 将参数默认值转化为configmap中保存的格式
func convertParams(paramsMap map[string]string) (map[string]string, error) {
	var result = make(map[string]string)
	for key, value := range paramsMap {
		bytes, err := json.Marshal(domain.ParamItem{
			Name:       key,
			Value:      value,
			UpdateTime: time.Now().Format(define.UnixTimeTemplate),
		})
		if err != nil {
			return nil, err
		}
		result[key] = string(bytes)
	}
	return result, nil
}
