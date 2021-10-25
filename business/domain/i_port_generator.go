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
 * @Description: 端口生成器
 */
type IPortGenerator interface {
	/**
	 * @Description: 获取一个新端口
	 * @return int
	 * @return error
	 */
	GetNextClusterExternalPort() (int, error)

	/**
	 * @Description: 检测Port是否被占用
	 * @param port
	 * @param rangeName
	 * @return bool
	 * @return error
	 */
	CheckPortUsed(port int, rangeName string) (bool, error)
}
