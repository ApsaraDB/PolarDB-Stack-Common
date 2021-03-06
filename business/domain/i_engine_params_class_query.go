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
 * @Description: 规格参数元数据查询
 */
type IEngineParamsClassQuery interface {
	/**
	 * @Description: 获得某规格特定内核参数
	 * @param engineType 引擎类型
	 * @param classKey 规格名称
	 * @return params 规格特定参数
	 * @return err
	 */
	GetClassParams(engineType EngineType, classKey string) (params map[string]string, err error)
}
