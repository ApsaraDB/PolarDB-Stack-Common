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


package define

import (
	"fmt"
	"strings"

	wfdefine "github.com/ApsaraDB/PolarDB-Stack-Workflow/define"
)

type InterruptError struct {
	Name      string
	Error     error  // 错误
	Reason    string // 出错原因
	Component string // 导致中断的组件
}

func (e *InterruptError) ToString() string {
	msg := e.Reason
	if e.Component != "" {
		msg = fmt.Sprintf("[%s] ", e.Component) + msg
	}
	if e.Error != nil {
		if e.Reason != "" {
			msg += ", error: "
		}
		msg += fmt.Sprintf("%v", e.Error)
	}
	return msg
}

func CreateInterruptError(e InterruptErrorEnum, err error) error {
	val := strings.Split(string(e), "|")
	var name, reason, component string
	if len(val) > 0 {
		name = val[0]
	}
	if len(val) > 1 {
		reason = val[1]
	}
	if len(val) > 2 {
		component = val[2]
	}
	interruptErr := &InterruptError{
		Name:      name,
		Reason:    reason,
		Component: component,
		Error:     err,
	}
	return wfdefine.NewInterruptError(name, interruptErr.ToString())
}

type InterruptErrorEnum string

const (
	// 格式: "中断错误类型|导致中断的组件|中断原因"
	InterruptPodLost        InterruptErrorEnum = "POD_LOST||pod is lost."
	InterruptPodUnScheduled InterruptErrorEnum = "POD_UNSCHEDULED||"
	InterruptDbStartFailed  InterruptErrorEnum = "DB_START_FAIL||"
	NoAvailableNode         InterruptErrorEnum = "NO_AVAILABLE_NODE||"
	TargetNodeUnavailable   InterruptErrorEnum = "TARGET_NODE_UNAVAILABLE||"
)
