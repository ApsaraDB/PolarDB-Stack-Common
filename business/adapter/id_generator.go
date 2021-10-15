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
	"errors"
	"fmt"
	"strconv"
	"sync"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/configuration"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/define"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/manager"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/k8sutil"
)

func NewIdGenerator(logger logr.Logger) *IdGenerator {
	return &IdGenerator{
		logger: logger.WithValues("component", "IdGenerator"),
	}
}

type IdGenerator struct {
	logger logr.Logger
	mutex  sync.Mutex
}

func (g *IdGenerator) GetNextClusterScopeHostInsIds(n int) ([]int, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	var result []int
	foundConfigMap, err := getInsIdConfigMap(g.logger)
	if err != nil {
		return nil, err
	}

	nextClusterScopeCustInsId, ok := foundConfigMap.Data[define.NextClusterScopeCustInsIdKey]
	if !ok {
		nextClusterScopeCustInsId = define.DefaultNextClusterScopeCustInsId
	}

	id, err := strconv.Atoi(nextClusterScopeCustInsId)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("strconv.Atoi error, err: %v", err))
	}

	for i := 0; i < n; i++ {
		result = append(result, id+i)
	}
	nextId := id + n

	strNextId := strconv.Itoa(nextId)
	foundConfigMap.Data[define.NextClusterScopeCustInsIdKey] = strNextId

	if err := manager.GetSyncClient().Update(context.TODO(), foundConfigMap); err != nil {
		return nil, errors.New(fmt.Sprintf("can not update NextClusterScopeCustInsId, err: %v", err))
	}

	return result, nil
}

func getInsIdConfigMap(logger logr.Logger) (*corev1.ConfigMap, error) {
	return k8sutil.GetConfigMap(configuration.GetConfig().InsIdRangeCmName, define.InsIdRangeCmNamespace, logger)
}
