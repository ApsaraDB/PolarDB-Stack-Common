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
	"github.com/go-logr/logr"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
)

func NewEngineParamsTemplateService(
	templateQuery domain.IEngineParamsTemplateQuery,
	classQuery domain.IEngineParamsClassQuery,
	logger logr.Logger,
) *EngineParamsTemplateService {
	return &EngineParamsTemplateService{
		templateQuery: templateQuery,
		classQuery:    classQuery,
		logger:        logger,
	}
}

type EngineParamsTemplateService struct {
	templateQuery domain.IEngineParamsTemplateQuery
	classQuery    domain.IEngineParamsClassQuery
	logger        logr.Logger
}

func (s *EngineParamsTemplateService) GetParamsTemplate(engineType domain.EngineType, className string) (*domain.ParamsTemplate, error) {
	result, err := domain.NewParamsTemplate(engineType, className, s.templateQuery, s.classQuery, s.logger)
	if err != nil {
		return nil, err
	}
	return result, nil
}
