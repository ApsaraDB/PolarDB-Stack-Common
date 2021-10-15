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
 * @Description: HostCluster元数据查询
 */
type IHostClusterQuery interface {
	/**
	 * @Description: 获得所有HostCluster
	 * @return []*domain.HostCluster
	 * @return error
	 */
	GetAllHostCluster() ([]*HostCluster, error)

	/**
	 * @Description: 根据名称获取某个HostCluster
	 * @param name
	 * @return *domain.HostCluster
	 * @return error
	 */
	GetHostCluster(name string) (*HostCluster, error)

	/**
	 * @Description: 获得本地HostCluster信息
	 * @param name
	 * @return *domain.HostCluster
	 * @return error
	 */
	GetLocalHostCluster() (*HostCluster, error)
}
