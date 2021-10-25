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

package configuration

import (
	corev1 "k8s.io/api/core/v1"
)

var (
	config *Config
)

// Config ...
type Config struct {
	DbClusterLogDir        string
	CmCpuReqLimit          string
	CmMemReqLimit          string
	HwCheck                bool
	ImagePullPolicy        corev1.PullPolicy
	RunPodInPrivilegedMode bool

	ClusterInfoDbInsTypeRw       string
	ClusterInfoDbInsTypeRo       string
	ClusterInfoDbInsTypeStandby  string
	ClusterInfoDbInsTypeDataMax  string
	ParamsTemplateName           string
	MinorVersionCmName           string
	ExternalPortRangeAnnotation  string
	ReservedPortRangesAnnotation string
	PortRangeCmName              string
	InsIdRangeCmName             string
	NodeMaintainLabelName        string
	AccountMetaClusterLabelName  string
	DbTypeLabel                  string
	ControllerManagerRoleName    string
	CmLogHostPath                string
	CmLogMountPath               string
}

func SetConfig(c *Config) {
	config = c
}

// GetConfig ...
func GetConfig() *Config {
	if config == nil {
		panic("config is nil, , need to call 'SetConfig(c)' first")
	}
	return config
}
