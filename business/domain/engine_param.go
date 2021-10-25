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

type ParamItem struct {
	Name        string `json:"name"`              // 参数名称
	Value       string `json:"value"`             // 参数值
	UpdateTime  string `json:"updateTime"`        // 更新时间
	Inoperative string `json:"inoperative_value"` // 未生效值，等待重启成效
}

type ChangedParamItemValue struct {
	OldValue    string
	NewValue    string
	NeedRestart bool
}
