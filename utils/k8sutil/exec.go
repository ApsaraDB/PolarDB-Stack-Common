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


package k8sutil

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/waitutil"

	"github.com/go-logr/logr"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/remotecommand"
)

var cmdRunnerCounter uint64 = 0

func cmdCounterGetter() uint64 {
	return atomic.AddUint64(&cmdRunnerCounter, 1)
}

func ExecInPodWithTimeout(ctx context.Context, command []string, containerName, podName, namespace string, timeout time.Duration, logger logr.Logger) (string, string, error) {
	var stdOutStr, stdErrStr string
	var latestErrInner error
	err := waitutil.PollImmediateWithContext(ctx, 3*time.Second, timeout, func() (bool, error) {
		var errInner error
		stdOutStr, stdErrStr, errInner = ExecInPod(command, containerName, podName, namespace, logger)
		if errInner != nil {
			latestErrInner = errInner
			if strings.Contains(errInner.Error(), "container not found") ||
				strings.Contains(errInner.Error(), "pod does not exist") {
				// unable to upgrade connection: container not found ("manager")
				logger.Error(errInner, "ExecInPod error, retry")
				return false, nil
			} else {
				return false, errInner
			}
		}
		return true, nil
	})
	if err != nil {
		return stdOutStr, stdErrStr, errors.Wrap(err, fmt.Sprintf("inner error: %v", latestErrInner))
	}
	return stdOutStr, stdErrStr, nil
}

func ExecInPod(command []string, containerName, podName, namespace string, logger logr.Logger) (string, string, error) {
	// command = []string{"bash", "-c", "a=bls c=d env; ls /var/log"}
	_, config, err := GetKubeClient()
	if err != nil {
		return "", "", err
	}
	client, err := corev1client.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	req := client.RESTClient().Post().
		Namespace(namespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)
	logger.V(8).Info("Request URL: " + req.URL().String())

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", fmt.Errorf("error while creating Executor: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})
	stdOutStr := stdout.String()
	stdErrStr := stderr.String()

	cmdOrder := cmdCounterGetter()

	stdOutWithoutEnter := strings.ReplaceAll(stdOutStr, "\n", "\\n")
	stdErrWithoutEnter := strings.ReplaceAll(stdErrStr, "\n", "\\n")

	logger.Info(fmt.Sprintf("execute cmd in pod."),
		"cmdOrder", cmdOrder,
		"pod", podName,
		"container", containerName,
		"cmd", command,
		"err", err,
		"std_err", stdErrWithoutEnter,
		"std_out", stdOutWithoutEnter,
	)
	return stdOutStr, stdErrStr, err
}
