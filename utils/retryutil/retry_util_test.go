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

package retryutil

import (
	"context"
	"errors"
	"testing"
	"time"

	"k8s.io/klog"
)

func TestRetryFunc(t *testing.T) {
	err := RetryFunc(context.TODO(), time.Second, 3, func() (bool, error) {
		return false, errors.New("test1 error")
	})
	klog.Error(err)

	err = RetryFunc(context.TODO(), time.Second, 3, func() (bool, error) {
		return true, errors.New("test2 error")
	})
	klog.Error(err)

	c, _ := context.WithTimeout(context.TODO(), 2*time.Second)
	err = RetryFunc(c, time.Second, 3, func() (bool, error) {
		return false, nil
	})
	klog.Error(err)

	err = RetryFunc(context.TODO(), time.Second, 3, func() (bool, error) {
		return true, nil
	})
	klog.Error(err)
}
