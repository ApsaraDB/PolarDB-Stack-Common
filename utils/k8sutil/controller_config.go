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

package k8sutil

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
)

type ControllerConfig struct {
	DisableSanCmd                bool
	HybridDeployment             bool
	CmHybridDeployment           bool
	CanUpdateNotChangeableParams bool // 是否可以更新不可变参数
	CowSnapshotSupport           bool
	EnableOnlinePromote          bool
	EnableVip                    bool
	EnablePodGc                  bool
	EnableCheckSum               bool
	EnableDegrade                bool
	EnableRedline                bool
	DegradeDaemonSet             []string
	DegradeDeployment            []string
	DisabledWfEvents             []string
	SshUser                      string
	SshPassword                  string
	ControllerNodeLabel          string
}

type GetControllerConfigPolicy struct {
	IsInited         bool
	LastQueryTime    *time.Time
	MaxQueryStepTime *time.Duration    //如果值为0表示只读一次,此值设置原则上不允许修改，否则会逻辑错乱
	LastValue        *ControllerConfig //最后一次结果暂存器
}

var ControllerConfigPolicy *GetControllerConfigPolicy

func init() {
	maxQueryStepTime := 30 * time.Minute
	ControllerConfigPolicy = &GetControllerConfigPolicy{
		IsInited:         false,
		MaxQueryStepTime: &maxQueryStepTime,
	}
}

func CheckPolicyIsReload(policy *GetControllerConfigPolicy) bool {
	if policy == nil || !policy.IsInited || policy.LastQueryTime == nil || policy.LastValue == nil {
		return true
	}

	//未设置允许时长，只读一次
	if policy.MaxQueryStepTime == nil || policy.MaxQueryStepTime.Seconds() == 0 {
		return false
	}

	stepSeconds := policy.MaxQueryStepTime.Seconds()
	realSeconds := time.Now().Sub(*policy.LastQueryTime).Seconds()
	if realSeconds >= stepSeconds {
		return true
	}
	return false
}

func GetControllerConfig(logger logr.Logger) (*ControllerConfig, error) {
	return GetControllerConfigWithPolicy(ControllerConfigPolicy, logger)
}

func GetControllerConfigWithPolicy(policy *GetControllerConfigPolicy, logger logr.Logger) (*ControllerConfig, error) {
	if policy != nil {
		reload := CheckPolicyIsReload(policy)
		if !reload {
			logger.Info("get controller-config from cache", "config", policy.LastValue)
			return policy.LastValue, nil
		}
	}
	logger.Info("reload controller-config")
	conf := &corev1.ConfigMap{}
	configMapKey := client.ObjectKey{
		Namespace: "kube-system",
		Name:      "controller-config",
	}
	if err := mgr.GetSyncClient().Get(context.TODO(), configMapKey, conf); err != nil {
		err = errors.Errorf("try to get controller config error: %v", err)
		logger.Error(err, "")
		return nil, err
	}
	var result = &ControllerConfig{}
	hybridDeployment, err := parseMapItemToBoolWithDefault(conf.Data, "hybridDeployment", true, logger)
	if err != nil {
		return nil, err
	}
	result.HybridDeployment = hybridDeployment

	disableSanCmd, err := parseMapItemToBoolWithDefault(conf.Data, "disableSanCmd", true, logger)
	if err != nil {
		return nil, err
	}
	result.DisableSanCmd = disableSanCmd

	cmHybridDeployment, err := parseMapItemToBoolWithDefault(conf.Data, "cmHybridDeployment", false, logger)
	if err != nil {
		return nil, err
	}
	result.CmHybridDeployment = cmHybridDeployment

	canUpdateNotChangeableParams, err := parseMapItemToBoolWithDefault(conf.Data, "canUpdateNotChangeableParams", false, logger)
	if err != nil {
		return nil, err
	}
	result.CanUpdateNotChangeableParams = canUpdateNotChangeableParams

	cowSnapshotSupport, err := parseMapItemToBoolWithDefault(conf.Data, "cowSnapshotSupport", false, logger)
	if err != nil {
		return nil, err
	}
	result.CowSnapshotSupport = cowSnapshotSupport

	enablePodGc, err := parseMapItemToBoolWithDefault(conf.Data, "enablePodGc", true, logger)
	if err != nil {
		return nil, err
	}
	result.EnablePodGc = enablePodGc

	result.EnableCheckSum, _ = parseMapItemToBoolWithDefault(conf.Data, "enableCheckSum", false, logger)

	result.EnableDegrade, _ = parseMapItemToBoolWithDefault(conf.Data, "enableDegrade", false, logger)

	degradeDaemonSet, err := parseMapItemToSlice(conf.Data, "degradeDaemonSet", logger)
	if err != nil {
		return nil, err
	}
	result.DegradeDaemonSet = degradeDaemonSet

	degradeDeployment, err := parseMapItemToSlice(conf.Data, "degradeDeployment", logger)
	if err != nil {
		return nil, err
	}
	result.DegradeDeployment = degradeDeployment

	disabledWfEvents, err := parseMapItemToSlice(conf.Data, "disabledWfEvents", logger)
	if err != nil {
		return nil, err
	}
	result.DisabledWfEvents = disabledWfEvents

	sshUser, err := parseMapItemToString(conf.Data, "sshUser", logger)
	if err != nil {
		return nil, err
	}
	result.SshUser = sshUser

	sshPassword, err := parseMapItemToString(conf.Data, "sshPassword", logger)
	if err != nil {
		return nil, err
	}
	result.SshPassword = sshPassword

	controllerNodeLabel, err := parseMapItemToString(conf.Data, "controllerNodeLabel", logger)
	if err != nil {
		return nil, err
	}
	result.ControllerNodeLabel = controllerNodeLabel

	enableOnlinePromote, err := parseMapItemToBoolWithDefault(conf.Data, "enableOnlinePromote", false, logger)
	if err != nil {
		return nil, err
	}
	result.EnableOnlinePromote = enableOnlinePromote

	enableRedline, err := parseMapItemToBoolWithDefault(conf.Data, "enableRedline", true, logger)
	if err != nil {
		return nil, err
	}
	result.EnableRedline = enableRedline

	enableVip, err := parseMapItemToBoolWithDefault(conf.Data, "enableVip", true, logger)
	if err != nil {
		return nil, err
	}
	result.EnableVip = enableVip
	if policy != nil {
		policy.LastValue = result
		policy.IsInited = true
		now := time.Now()
		policy.LastQueryTime = &now
	}

	return result, nil
}

func parseMapItemToBoolWithDefault(data map[string]string, key string, defaultValue bool, logger logr.Logger) (bool, error) {
	valueStr, ok := data[key]
	if !ok {
		logger.Error(errors.New(fmt.Sprintf("key [%s] not found from map %v", key, data)), "")
	}
	if valueStr == "" {
		return defaultValue, nil
	}
	strTrim := strings.ToUpper(strings.TrimSpace(valueStr))
	if "1" == strTrim || "T" == strTrim || "Y" == strTrim || "TRUE" == strTrim || "YES" == strTrim {
		return true, nil
	} else {
		if defaultValue {
			//默认为true，预期之外的字符串应视为true，即言下之意，False也必须在枚举范围之内
			return "0" != strTrim && "F" != strTrim && "N" != strTrim && "FALSE" != strTrim && "NO" != strTrim, nil
		} else {
			//默认为true ,预期之外的字符串应视为false
			return false, nil
		}
	}
}

func parseMapItemToString(data map[string]string, key string, logger logr.Logger) (string, error) {
	value, ok := data[key]
	if !ok {
		err := errors.Errorf("key [%s] not found from map %v", key, data)
		logger.Error(err, "")
		return "", err
	}
	return value, nil
}

func parseMapItemToSlice(data map[string]string, key string, logger logr.Logger) ([]string, error) {
	valueStr, ok := data[key]
	if !ok {
		err := errors.Errorf("key [%s] not found from map %v", key, data)
		logger.Error(err, "")
		return nil, err
	}
	return strings.Split(valueStr, ","), nil
}
