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

/**
 * @Description: 小版本元数据查询
 */
type IMinorVersionQuery interface {
	/**
	 * @Description: 获取所有内核小版本信息
	 * @param engineType 引擎类型
	 * @return latestVersion 最新版本
	 * @return versionList 内核小版本列表
	 * @return err
	 */
	GetMinorVersions(engineType EngineType) (latestVersion string, versionList []*MinorVersion, err error)

	/**
	 * @Description: 获得最新的内核小版本信息
	 * @param engineType
	 * @return version
	 * @return err
	 */
	GetLatestMinorVersion(engineType EngineType) (version *MinorVersion, err error)

	/**
	 * @Description: 获得内核小版本信息
	 * @param engineType
	 * @param versionName
	 * @return version
	 * @return err
	 */
	GetMinorVersion(engineType EngineType, versionName string) (version *MinorVersion, err error)
}
