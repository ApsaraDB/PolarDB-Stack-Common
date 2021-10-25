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

import "github.com/ApsaraDB/PolarDB-Stack-Common/utils"

type Account struct {
	Account        string `json:"account"`
	Password       string `json:"password"`
	PrivilegedType int    `json:"priviledge_type"`
}

func CreateAccountWithRandPassword(account string, privilegedType int) *Account {
	return &Account{
		Account:        account,
		Password:       utils.RandPassword(),
		PrivilegedType: privilegedType,
	}
}

func CreateAccountWithPassword(account, password string, privilegedType int) *Account {
	return &Account{
		Account:        account,
		Password:       password,
		PrivilegedType: privilegedType,
	}
}
