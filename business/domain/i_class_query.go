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
 * @Description: 规格元数据查询
 */
type IClassQuery interface {
	/**
	 * @Description: 获取所有规格清单
	 * @param engineType 引擎类型
	 * @param classKey 规格名称，如果为空则获取所有规格，不为空则获取指定规格
	 * @return classList
	 * @return err
	 */
	GetClasses(engineType EngineType, classKey string) (classList []*EngineClass, err error)
}
