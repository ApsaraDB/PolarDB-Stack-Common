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
 * @Description: 引擎内置账户元数据持久化
 */
type IAccountRepository interface {
	/**
	 * @Description: 保存账户元数据
	 * @param engine
	 * @param account
	 * @return error
	 */
	EnsureAccountMeta(cluster *DbClusterBase, account *Account) error

	/**
	 * @Description: 获取账户元数据
	 * @param clusterName
	 * @param namespace
	 * @return accounts
	 * @return error
	 */
	GetAccounts(clusterName, namespace string) (accounts map[string]*Account, err error)
}
