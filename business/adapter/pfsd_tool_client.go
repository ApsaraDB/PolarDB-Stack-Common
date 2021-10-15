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


package adapter

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/business/domain"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/define"

	"github.com/pkg/errors"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/k8sutil"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/waitutil"
)

func NewPfsdToolClient(logger logr.Logger) *PfsdToolClient {
	return &PfsdToolClient{
		logger: logger.WithValues("component", "PfsdToolClient"),
	}
}

type PfsdToolClient struct {
	ins       *domain.DbIns
	resources map[string]*domain.InstanceResource
	volumeId  string
	inited    bool
	logger    logr.Logger
}

func (m *PfsdToolClient) Init(
	ins *domain.DbIns,
	resources map[string]*domain.InstanceResource,
	volumeId string,
) {
	m.ins = ins
	m.resources = resources
	m.volumeId = volumeId
	m.inited = true
}

var notInitPfsdClientError = errors.New("pfsd tool client not init.")

/**
 * @Description: 启动pfsd
 * @return error
 */
func (m *PfsdToolClient) StartPfsd(ctx context.Context) error {
	if !m.inited {
		return notInitPfsdClientError
	}
	_, _, err := m.execCmdInPfsdToolContainer(ctx, "start_instance", m.getPfsdStartConfig())
	return err
}

func (m *PfsdToolClient) getPfsdStartConfig() string {
	config := m.resources[define.ContainerNamePfsd].Config
	param := fmt.Sprintf("%s -p mapper_%s -e %s", config, m.volumeId, m.ins.InsId)
	return param
}

func (m *PfsdToolClient) execCmdInPfsdToolContainer(ctx context.Context, action, param string) (stdout string, stderr string, err error) {
	command := []string{"bash", "-c", fmt.Sprintf("srv_opr_action=%s python2 /docker_script/entry_oper.py %s", action, param)}
	execFunc := func() (stdout string, stderr string, err error) {
		stdout, stderr, err = k8sutil.ExecInPod(command, define.ContainerNamePfsdTool, m.ins.ResourceName, m.ins.ResourceNamespace, m.logger)
		return
	}

	var innerErr error
	err = waitutil.PollImmediateWithContext(ctx, 3*time.Second, 60*time.Second, func() (bool, error) {
		stdout, stderr, innerErr = execFunc()
		if err == nil {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		if innerErr != nil {
			err = errors.Wrap(err, fmt.Sprintf("%v", innerErr))
		}
		m.logger.Error(err, "execute cmd failed", "cmd", command)
	}
	return
}
