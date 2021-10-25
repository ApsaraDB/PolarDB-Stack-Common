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

package domain

import (
	"fmt"

	"k8s.io/klog/klogr"

	"github.com/go-logr/logr"

	"github.com/pkg/errors"
)

func NewParamsTemplate(
	engineType EngineType,
	classKey string,
	paramsTemplateRepo IEngineParamsTemplateQuery,
	paramsClassRepo IEngineParamsClassQuery,
	logger logr.Logger,
) (result *ParamsTemplate, err error) {
	if paramsTemplateRepo == nil {
		return nil, errors.New("params template query is nil")
	}
	if paramsClassRepo == nil {
		return nil, errors.New("params class query is nil")
	}
	result = &ParamsTemplate{
		EngineType:         engineType,
		ClassKey:           classKey,
		paramsTemplateRepo: paramsTemplateRepo,
		paramsClassRepo:    paramsClassRepo,
	}
	if logger != nil {
		result.logger = logger.WithName(string(engineType)).WithName(classKey)
	} else {
		result.logger = klogr.New().WithName(string(engineType)).WithName(classKey)
	}
	if err = result.load(); err != nil {
		return nil, err
	}
	return result, nil
}

type ParamsTemplate struct {
	EngineType           EngineType
	ClassKey             string
	templateParams       []*ParamTemplateItem
	classParams          map[string]string
	mergedTemplateParams []*ParamTemplateItem
	paramsTemplateRepo   IEngineParamsTemplateQuery
	paramsClassRepo      IEngineParamsClassQuery
	logger               logr.Logger
}

type ParamTemplateItem struct {
	Name             string `json:"name"`              // 参数名
	DefaultValue     string `json:"default_value"`     // 默认值
	IsDynamic        int    `json:"is_dynamic"`        // 是否需要重启
	IsVisible        int    `json:"is_visible"`        // 控制台是否可见
	IsUserChangeable int    `json:"is_user_changable"` // 用户是否可更改
	Optional         string `json:"optional"`          // 正则表达式
	Unit             string `json:"unit"`              //
	DivideBase       string `json:"divide_base"`       //
	IsDeleted        int    `json:"is_deleted"`        //
	Comment          string `json:"comment"`           //
}

/**
 * @Description: 获得合并了规格默认参数之后的参数模板
 * @receiver t *ParamsTemplate
 * @return []*ParamTemplateItem
 */
func (t *ParamsTemplate) GetMergedTemplateParams() []*ParamTemplateItem {
	return t.mergedTemplateParams
}

/**
 * @Description: 获得某规格默认参数值
 * @receiver t *ParamsTemplate
 * @return map[string]string
 */
func (t *ParamsTemplate) GetClassInitParams() map[string]string {
	templateParams := t.getParamsTemplateDefaultValues()
	// 规格特定参数覆盖参数模板
	for k, v := range t.classParams {
		templateParams[k] = v
	}
	return templateParams
}

func (t *ParamsTemplate) load() error {
	templateParams, err := t.paramsTemplateRepo.GetTemplateParams(t.EngineType)
	if err != nil {
		return err
	}
	t.templateParams = templateParams
	classParams, err := t.paramsClassRepo.GetClassParams(t.EngineType, t.ClassKey)
	if err != nil {
		return err
	}
	t.classParams = classParams
	t.mergedTemplateParams = t.getClassParamsTemplate()
	return nil
}

func (t *ParamsTemplate) getClassParamsTemplate() []*ParamTemplateItem {
	var mergedTemplateParams []*ParamTemplateItem
	for _, templateParam := range t.templateParams {
		mergedTemplateParams = append(mergedTemplateParams, &ParamTemplateItem{
			Name:             templateParam.Name,
			DefaultValue:     templateParam.DefaultValue,
			IsDynamic:        templateParam.IsDynamic,
			IsVisible:        templateParam.IsVisible,
			IsUserChangeable: templateParam.IsUserChangeable,
			Optional:         templateParam.Optional,
			Unit:             templateParam.Unit,
			DivideBase:       templateParam.DivideBase,
			IsDeleted:        templateParam.IsDeleted,
			Comment:          templateParam.Comment,
		})
	}
	for k, v := range t.classParams {
		haveItem := false
		for _, templateParam := range mergedTemplateParams {
			if templateParam.Name == k {
				templateParam.DefaultValue = v
				haveItem = true
				break
			}
		}
		// 如果规格中的参数不包含在参数模板中，则作为不可变参数加入到参数模板
		if !haveItem {
			mergedTemplateParams = append(mergedTemplateParams, &ParamTemplateItem{
				Name:             k,
				DefaultValue:     v,
				IsDynamic:        1,
				IsVisible:        0,
				IsUserChangeable: 0,
				IsDeleted:        0,
			})
		}
	}
	return mergedTemplateParams
}

func (t *ParamsTemplate) getParamsTemplateDefaultValues() map[string]string {
	result := make(map[string]string)
	for _, param := range t.templateParams {
		result[fmt.Sprintf("%v", param.Name)] = fmt.Sprintf("%v", param.DefaultValue)
	}
	return result
}
