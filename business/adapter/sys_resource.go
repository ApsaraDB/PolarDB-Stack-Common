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
	"strconv"

	"github.com/go-logr/logr"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	"github.com/ApsaraDB/PolarDB-Stack-Common/define"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"
)

type SysResourceConfig struct {
	Single        SysResource `yaml:"single"`
	ReadWriteMany SysResource `yaml:"readWriteMany"`
}

type SysResource struct {
	Manager  *SysResourceQuantity `yaml:"manager"`
	PfsdTool *SysResourceQuantity `yaml:"pfsdTool" binding:"omitempty"`
	Pfsd     *SysResourceQuantity `yaml:"pfsd" binding:"omitempty"`
}

type SysResourceQuantity struct {
	Limits   ResourceQuantity `yaml:"limits"`
	Requests ResourceQuantity `yaml:"requests"`
}

type ResourceQuantity struct {
	CPU    resource.Quantity `yaml:"cpu"`
	Memory resource.Quantity `yaml:"memory"`
}

func GetSysResConfig(logger logr.Logger) (conf *SysResourceConfig, err error) {
	cm, err := k8sutil.GetConfigMap(define.InstanceSystemResourcesCmName, define.InstanceSystemResourcesCmNamespace, logger)
	if err != nil {
		err = errors.Wrap(err, "failed to get instance-system-resources configmap")
		return
	}
	enableResourceShare, err := strconv.ParseBool(cm.Data["enableResourceShare"])
	if err != nil {
		err = errors.Wrap(err, "fail to string transfer bool")
		return nil, err
	}
	key := "shared"
	if !enableResourceShare {
		key = "normal"
	}
	resourceConfStr := cm.Data[key]
	conf = &SysResourceConfig{}
	err = yaml.Unmarshal([]byte(resourceConfStr), conf)
	if err != nil {
		err = errors.Wrap(err, "yaml Unmarshal err")
		return nil, err
	}
	return conf, nil
}
