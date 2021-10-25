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

package waitutil

import (
	"context"
	"time"

	"github.com/pkg/errors"

	w "k8s.io/apimachinery/pkg/util/wait"
)

var ErrWaitCancelled = errors.New("cancelled while waiting for the condition")

func PollInfinite(ctx context.Context, interval time.Duration, condition w.ConditionFunc) error {
	return WaitFor(poller(interval, 0), condition, ctx.Done())
}

func PollImmediateWithContext(ctx context.Context, interval, timeout time.Duration, condition w.ConditionFunc) error {
	return pollImmediateInternal(poller(interval, timeout), condition, ctx)
}

func poller(interval, timeout time.Duration) w.WaitFunc {
	return w.WaitFunc(func(done <-chan struct{}) <-chan struct{} {
		ch := make(chan struct{})

		go func() {
			defer close(ch)

			tick := time.NewTicker(interval)
			defer tick.Stop()

			var after <-chan time.Time
			if timeout != 0 {
				// time.After is more convenient, but it
				// potentially leaves timers around much longer
				// than necessary if we exit early.
				timer := time.NewTimer(timeout)
				after = timer.C
				defer timer.Stop()
			}

			for {
				select {
				case <-tick.C:
					// If the consumer isn't ready for this signal drop it and
					// check the other channels.
					select {
					case ch <- struct{}{}:
					default:
					}
				case <-after:
					return
				case <-done:
					return
				}
			}
		}()

		return ch
	})
}

func pollImmediateInternal(wait w.WaitFunc, condition w.ConditionFunc, ctx context.Context) error {
	done, err := condition()
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	return WaitFor(wait, condition, ctx.Done())
}

func WaitFor(wait w.WaitFunc, fn w.ConditionFunc, done <-chan struct{}) error {
	stopCh := make(chan struct{})
	defer close(stopCh)
	c := wait(stopCh)
	for {
		select {
		case _, open := <-c:
			ok, err := fn()
			if err != nil {
				return err
			}
			if ok {
				return nil
			}
			if !open {
				return w.ErrWaitTimeout
			}
		case <-done:
			return ErrWaitCancelled
		}
	}
}
