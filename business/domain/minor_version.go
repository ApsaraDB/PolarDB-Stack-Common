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
	"fmt"

	"github.com/pkg/errors"
)

type EngineType string

const (
	EngineTypeSingle  EngineType = "Single"
	EngineTypeRwo     EngineType = "Rwo"
	EngineTypeDataMax EngineType = "standbyDataMax"
	EngineTypeStandby EngineType = "Standby"
)

func NewMinorVersion(engineType EngineType, repository IMinorVersionQuery) (result *MinorVersions, err error) {
	if engineType == "" {
		return nil, errors.New("engineType is empty")
	}
	if repository == nil {
		return nil, errors.New("minorVersionQuery is nil")
	}
	result = &MinorVersions{
		EngineType:    engineType,
		latestVersion: "",
		minorVersions: nil,
		repository:    repository,
	}

	return result, nil
}

type MinorVersions struct {
	EngineType    EngineType
	latestVersion string
	minorVersions []*MinorVersion
	repository    IMinorVersionQuery
}

type MinorVersion struct {
	Name   string
	Images map[string]string
}

/**
 * @Description: 获取内核小版本列表
 * @receiver v
 * @return list
 * @return err
 */
func (v *MinorVersions) GetMinorVersionList() (list []*MinorVersion, err error) {
	if v.minorVersions == nil || len(v.minorVersions) == 0 {
		v.latestVersion, v.minorVersions, err = v.repository.GetMinorVersions(v.EngineType)
		if err != nil {
			return
		}
	}
	list = v.minorVersions
	return
}

/**
 * @Description: 获取最新内核小版本信息
 * @receiver v *MinorVersions
 * @param engineType
 * @return *MinorVersion
 */
func (v *MinorVersions) GetLatestMinorVersion() (version *MinorVersion, err error) {
	if v.minorVersions == nil || len(v.minorVersions) == 0 {
		return v.repository.GetLatestMinorVersion(v.EngineType)
	}
	for _, version := range v.minorVersions {
		if version.Name == v.latestVersion {
			return version, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("GetLatestMinorVersion err: %s configmap not found", v.latestVersion))
}
