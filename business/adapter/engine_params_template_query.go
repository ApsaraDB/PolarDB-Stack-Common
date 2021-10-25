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
	"strings"

	"github.com/ApsaraDB/PolarDB-Stack-Common/configuration"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	"github.com/ApsaraDB/PolarDB-Stack-Common/define"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"
)

var classSuffix = map[domain.EngineType]string{
	domain.EngineTypeDataMax: "-dm",
	domain.EngineTypeRwo:     "-rwo",
	domain.EngineTypeSingle:  "",
	domain.EngineTypeStandby: "-rwo",
}

func NewEngineParamsTemplateQuery(logger logr.Logger) *EngineParamsTemplateQuery {
	return &EngineParamsTemplateQuery{
		logger: logger.WithValues("component", "EngineParamsTemplateQuery"),
	}
}

type EngineParamsTemplateQuery struct {
	logger logr.Logger
}

func (q *EngineParamsTemplateQuery) GetTemplateParams(engineType domain.EngineType) (params []*domain.ParamTemplateItem, err error) {
	whiteListCmName := configuration.GetConfig().ParamsTemplateName + "-white-list" + classSuffix[engineType]
	whiteListCm, err := k8sutil.GetConfigMap(whiteListCmName, define.ParamsTemplateNamespace, q.logger)
	var whiteList []string
	var extendTemplates []*domain.ParamTemplateItem

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	} else {
		if canUpdateParamsStr, ok := whiteListCm.Data["can_update_params"]; ok {
			whiteList = strings.Split(canUpdateParamsStr, "|")
		}
		if extendTemplatesStr, ok := whiteListCm.Data["extend_mycnf_template"]; ok {
			if err = yaml.Unmarshal([]byte(extendTemplatesStr), &extendTemplates); err != nil {
				q.logger.Error(err, "parse extend_mycnf_template failed")
				// 解析失败忽略
			}
		}
	}

	foundConfigMap, err := k8sutil.GetConfigMap(configuration.GetConfig().ParamsTemplateName+classSuffix[engineType], define.ParamsTemplateNamespace, q.logger)
	if err != nil {
		return nil, err
	}
	var result []*domain.ParamTemplateItem
	if err = yaml.Unmarshal([]byte(foundConfigMap.Data["mycnf_template"]), &result); err != nil {
		return nil, err
	}
	if whiteList != nil && len(whiteList) > 0 {
		for _, item := range result {
			if utils.ContainsString(whiteList, item.Name, nil) {
				item.IsUserChangeable = 1
			}
		}
	}
	if extendTemplates != nil && len(extendTemplates) > 0 {
		for _, extendTemplate := range extendTemplates {
			if t := findTemplate(extendTemplate.Name, result); t == nil {
				result = append(result, extendTemplate)
			}
		}
	}
	return result, nil
}

func findTemplate(name string, allTemplate []*domain.ParamTemplateItem) *domain.ParamTemplateItem {
	for _, template := range allTemplate {
		if template.Name == name {
			return template
		}
	}
	return nil
}
