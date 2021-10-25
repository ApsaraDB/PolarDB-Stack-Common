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

package service

import (
	"context"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewCmCreatorService(
	cmCreator domain.IClusterManagerCreator,
) *CmCreatorService {
	return &CmCreatorService{
		cmCreator: cmCreator,
	}
}

type CmCreatorService struct {
	cmCreator domain.IClusterManagerCreator
}

/**
 * @Description: 创建ClusterManager
 * @param ctx
 * @param kubeObj dbcluster/standby k8s对象
 * @param logicInsId
 * @param rwPhyId
 * @param cmImage
 * @param port
 * @return error
 */
func (s *CmCreatorService) CreateClusterManager(ctx context.Context, kubeObj metav1.Object, workMode domain.CmWorkMode, logicInsId, rwPhyId, cmImage string, port int, pluginConf map[string]interface{}, consensusPort int) error {
	return s.cmCreator.CreateClusterManager(ctx, kubeObj, workMode, logicInsId, rwPhyId, cmImage, port, pluginConf, consensusPort)
}
