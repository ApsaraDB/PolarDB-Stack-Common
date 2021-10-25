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
 * @Description: 引擎参数元数据
 */
type IEngineParamsRepository interface {
	/**
	 * @Description: 保存用户设置的参数
	 * @param engine
	 * @param initParams
	 * @return error
	 */
	SaveUserParams(engine *DbClusterBase, initParams map[string]string) error

	/**
	 * @Description: 获取用户设置的参数
	 * @param engine
	 * @return map[string]string
	 * @return string 最后一次成功刷参的时间
	 * @return error
	 */
	GetUserParams(engine *DbClusterBase) (map[string]*ParamItem, string, error)

	/**
	 * @Description: 保存生效参数，初始化时使用，后面由cm更新
	 * @param engine
	 * @param initParams
	 * @return error
	 */
	SaveRunningParams(engine *DbClusterBase, initParams map[string]string) error

	/**
	 * @Description: 获取运行中的参数
	 * @param engine
	 * @return map[string]string
	 * @return error
	 */
	GetRunningParams(engine *DbClusterBase) (map[string]*ParamItem, error)

	/**
	 * @Description: 保存最后刷参成功时间
	 * @param dbClusterName
	 * @return map[string]string
	 * @return error
	 */
	SaveLatestFlushTime(engine *DbClusterBase) error

	/**
	 * @Description: 更新UserParam
	 * @param dbClusterName
	 * @return map[string]string
	 * @return error
	 */
	UpdateUserParams(engine *DbClusterBase, userParams map[string]*EffectiveUserParamValue) error
}
