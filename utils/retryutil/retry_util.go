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
	"fmt"
	"time"
)

type RetryError struct {
	n         int
	lastError error
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("still failing after %d retries, last error: %v", e.n, e.lastError)
}

func IsRetryFailure(err error) bool {
	_, ok := err.(*RetryError)
	return ok
}

type ConditionInterface interface {
	ConditionFunc() (bool, error)
}

// Retry retries f every interval until after maxRetries.
// The interval won't be affected by how long f takes.
// For example, if interval is 3s, f takes 1s, another f will be called 2s later.
// However, if f takes longer than interval, it will be delayed.
func Retry(ctx context.Context, interval time.Duration, maxRetries int, ci ConditionInterface) error {
	return RetryFunc(ctx, interval, maxRetries, ci.ConditionFunc)
}

func RetryFunc(ctx context.Context, interval time.Duration, maxRetries int, ConditionFunc func() (bool, error)) error {
	if maxRetries <= 0 {
		return fmt.Errorf("maxRetries (%d) should be > 0", maxRetries)
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	var lastErr error
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("the context timeout has been reached")
		case <-tick.C:
			ok, err := ConditionFunc()
			if err == nil && ok {
				return nil
			}
			if err != nil {
				lastErr = err
				if ok {
					return err
				}
			}
			if i == maxRetries {
				return &RetryError{maxRetries, lastErr}
			}
		}
	}
}
