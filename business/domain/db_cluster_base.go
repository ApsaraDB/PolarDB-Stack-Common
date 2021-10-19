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
	"context"
	"fmt"
	"regexp"

	"github.com/pkg/errors"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"

	"github.com/go-logr/logr"

	"github.com/ApsaraDB/PolarDB-Stack-Common/define"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils"
)

type DbClusterType string

const (
	DbClusterTypeStandby DbClusterType = "Standby"
	DbClusterTypeDataMax DbClusterType = "Datamax"
	DbClusterTypeMaster  DbClusterType = "Master"
)

// DB集群基类
type DbClusterBase struct {
	Name                    string
	Namespace               string
	Description             string
	LogicInsId              string
	ClusterStatus           string
	ClusterManager          *ClusterManagerInfo
	ImageInfo               *ImageInfo
	ClassInfo               *ClassInfo
	Resources               map[string]*InstanceResource
	DbClusterType           DbClusterType
	Interrupt               bool
	InterruptMsg            string
	InterruptReason         string
	EngineType              EngineType
	ResourceVersion         string
	PrimarySystemIdentifier string
	UseModifyClass          bool
	UseUpgradeImageInfo     bool
	OldClassInfo            *ClassInfo
	OldImageInfo            *ImageInfo

	ParamsTemplateQuery   IEngineParamsTemplateQuery
	ParamsClassQuery      IEngineParamsClassQuery
	EngineParamsRepo      IEngineParamsRepository
	MinorVersionQuery     IMinorVersionQuery
	AccountRepo           IAccountRepository
	HostClusterQuery      IHostClusterQuery
	IdGenerator           IIdGenerator
	PortGenerator         IPortGenerator
	ClassQuery            IClassQuery
	ClusterManagerClient  IClusterManagerClient
	ClusterManagerRemover IClusterManagerRemover

	Logger logr.Logger
	inited bool
}

type ClassInfo struct {
	ClassName string
	CPU       string
	Memory    string
}

type ImageInfo struct {
	Version string
	Images  map[string]string
}

type ClusterManagerInfo struct {
	Port          int
	ConsensusPort int
}

type EffectiveUserParamValue struct {
	Value             string
	NeedFlush         bool
	RunningUpdateTime string
}

var clusterBaseNotInitError = errors.New("cluster base not init.")

func (dbCluster *DbClusterBase) GetClusterResourceName() string {
	return fmt.Sprintf("%s-%s", dbCluster.Name, dbCluster.LogicInsId)
}

func (dbCluster *DbClusterBase) Init(
	paramsTemplateQuery IEngineParamsTemplateQuery,
	paramsClassQuery IEngineParamsClassQuery,
	paramsRepo IEngineParamsRepository,
	minorVersionQuery IMinorVersionQuery,
	accountRepo IAccountRepository,
	idGenerator IIdGenerator,
	portGenerator IPortGenerator,
	classQuery IClassQuery,
	clusterManagerClient IClusterManagerClient,
	hostClusterQuery IHostClusterQuery,
	clusterManagerRemover IClusterManagerRemover,
	logger logr.Logger,
) {
	dbCluster.Logger = logger
	dbCluster.ParamsClassQuery = paramsClassQuery
	dbCluster.ParamsTemplateQuery = paramsTemplateQuery
	dbCluster.EngineParamsRepo = paramsRepo
	dbCluster.MinorVersionQuery = minorVersionQuery
	dbCluster.AccountRepo = accountRepo
	dbCluster.IdGenerator = idGenerator
	dbCluster.PortGenerator = portGenerator
	dbCluster.ClassQuery = classQuery
	dbCluster.ClusterManagerClient = clusterManagerClient
	dbCluster.HostClusterQuery = hostClusterQuery
	dbCluster.ClusterManagerRemover = clusterManagerRemover

	dbCluster.inited = true
}

/**
 * @Description: 初始化规格元数据
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *DbClusterBase) InitClass() error {
	if !dbCluster.inited {
		return clusterNotInitError
	}
	if result, err := NewEngineClasses(dbCluster.EngineType, dbCluster.ClassQuery); err != nil {
		return err
	} else if c, err := result.GetClass(dbCluster.ClassInfo.ClassName); err != nil {
		return err
	} else {
		dbCluster.Resources = c.Resource
	}
	return nil
}

/**
 * @Description: 初始化版本元数据
 * @receiver dm
 * @return error
 */
func (dbCluster *DbClusterBase) InitVersion() (err error) {
	if !dbCluster.inited {
		return clusterNotInitError
	}
	var version *MinorVersion
	if dbCluster.ImageInfo == nil || dbCluster.ImageInfo.Version == "" {
		version, err = dbCluster.MinorVersionQuery.GetLatestMinorVersion(dbCluster.EngineType)
		if err != nil {
			return err
		}
	} else {
		version, err = dbCluster.MinorVersionQuery.GetMinorVersion(dbCluster.EngineType, dbCluster.ImageInfo.Version)
		if err != nil {
			return err
		}
	}
	dbCluster.ImageInfo.Version = version.Name
	dbCluster.ImageInfo.Images = version.Images
	return nil
}

/**
 * @Description: 初始化引擎参数
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *DbClusterBase) InitEngineParams() error {
	if !dbCluster.inited {
		return clusterNotInitError
	}
	paramsTemplate, err := NewParamsTemplate(dbCluster.EngineType, dbCluster.ClassInfo.ClassName, dbCluster.ParamsTemplateQuery, dbCluster.ParamsClassQuery, dbCluster.Logger)
	if err != nil {
		return err
	}
	initParams := paramsTemplate.GetClassInitParams()
	err = dbCluster.EngineParamsRepo.SaveUserParams(dbCluster, initParams)
	if err != nil {
		return err
	}
	err = dbCluster.EngineParamsRepo.SaveRunningParams(dbCluster, initParams)
	if err != nil {
		return err
	}
	return nil
}

/**
 * @Description: 变配时应用新规格参数，或者参数模板不可变参数默认值更新后应用新默认值
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *DbClusterBase) ModifyClassParams() error {
	if !dbCluster.inited {
		return clusterNotInitError
	}
	canUpdateNotChangeableParams := GetCanUpdateNotChangeableParams(dbCluster.Logger)
	userParams, changed, _, err := dbCluster.GetEffectiveUserParams(false, canUpdateNotChangeableParams)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return dbCluster.EngineParamsRepo.UpdateUserParams(dbCluster, userParams)
}

/**
 * @Description: 保存最后刷参成功时间
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *DbClusterBase) SaveLatestFlushTime() error {
	if !dbCluster.inited {
		return clusterNotInitError
	}
	return dbCluster.EngineParamsRepo.SaveLatestFlushTime(dbCluster)
}

/**
 * @Description: 初始化账户元数据
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *DbClusterBase) InitAccount() error {
	if !dbCluster.inited {
		return clusterNotInitError
	}
	err := dbCluster.AccountRepo.EnsureAccountMeta(dbCluster, CreateAccountWithRandPassword("aurora", 7))
	if err != nil {
		return err
	}
	err = dbCluster.AccountRepo.EnsureAccountMeta(dbCluster, CreateAccountWithRandPassword("replicator", 15))
	if err != nil {
		return err
	}
	return nil
}

/**
 * @Description: 禁用HA
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *DbClusterBase) DisableHA(ctx context.Context) error {
	if err := dbCluster.ClusterManagerClient.InitWithLocalDbCluster(ctx, dbCluster.Namespace, dbCluster.Name, true); err != nil {
		return err
	}
	return dbCluster.ClusterManagerClient.DisableHA(ctx)
}

/**
 * @Description: 启用HA
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *DbClusterBase) EnableHA(ctx context.Context) error {
	if err := dbCluster.ClusterManagerClient.InitWithLocalDbCluster(ctx, dbCluster.Namespace, dbCluster.Name, true); err != nil {
		return err
	}
	return dbCluster.ClusterManagerClient.EnableHA(ctx)
}

func (dbCluster *DbClusterBase) EnsureCmAffinity(ctx context.Context, rwHostName string) error {
	return dbCluster.ClusterManagerClient.EnsureAffinity(ctx, dbCluster.Namespace, dbCluster.Name, rwHostName)
}

func (dbCluster *DbClusterBase) UpgradeCmVersion(ctx context.Context, cmImage string) error {
	return dbCluster.ClusterManagerClient.UpgradeVersion(ctx, dbCluster.Namespace, dbCluster.Name, cmImage)
}

/**
 * @Description: 执行刷参
 * @receiver ins
 * @return error
 */
func (dbCluster *DbClusterBase) FlushParams(ins *DbIns, ctx context.Context) error {
	if !dbCluster.inited {
		return clusterNotInitError
	}
	canUpdateNotChangeableParams := GetCanUpdateNotChangeableParams(dbCluster.Logger)
	params, _, needRestart, err := dbCluster.GetEffectiveParams(false, canUpdateNotChangeableParams)
	if err != nil {
		return err
	}
	return ins.FlushParams(ctx, params, needRestart)
}

/**
 * @Description: 删除DB集群对应的ClusterManager及其元数据
 * @receiver dbCluster
 * @return error
 */
func (dbCluster *DbClusterBase) DeleteCm(ctx context.Context) error {
	return dbCluster.ClusterManagerRemover.Remove(dbCluster.Name, dbCluster.Namespace, ctx)
}

/**
 * @Description: 获得最终生效的参数，创建实例和刷参场景，参数合并规则参考 https://yuque.antfin.com/pg-hdb-dev/polardb-o-machine/vozb6o
 * @receiver dbCluster
 * @param inoperative 是否使未生效值生效
 * @param canUpdateNotChangeableParams 是否可以更改不可变参数
 * @return paramItems 最终生效参数
 * @return changed 参数是否有变更
 * @return needRestart 是否需要重启
 * @return err
 */
func (dbCluster *DbClusterBase) GetEffectiveParams(inoperative bool, canUpdateNotChangeableParams bool) (paramItems map[string]string, changed bool, needRestart bool, err error) {
	result := make(map[string]string)
	changedParams := make(map[string]ChangedParamItemValue)

	paramsTemplate, err := NewParamsTemplate(dbCluster.EngineType, dbCluster.ClassInfo.ClassName, dbCluster.ParamsTemplateQuery, dbCluster.ParamsClassQuery, dbCluster.Logger)
	if err != nil {
		return nil, false, false, err
	}
	templateParams := paramsTemplate.getClassParamsTemplate()

	// 按照最后更新时间合并running-params、user-params
	userParams, latestFlushTimeStr, err := dbCluster.EngineParamsRepo.GetUserParams(dbCluster)
	if err != nil {
		return nil, false, false, err
	}
	runningParams, err := dbCluster.EngineParamsRepo.GetRunningParams(dbCluster)
	if err != nil {
		return nil, false, false, err
	}
	for paramKey, runningParamValue := range runningParams {
		if utils.ContainsString(define.IgnoreParams, paramKey, nil) {
			continue
		}
		userParamValue, ok := userParams[paramKey]
		oldValue := runningParamValue.Value
		if ok {
			oldValue := runningParamValue.Value
			// 用户更改过，以用户值为准
			if userParamValue.UpdateTime > runningParamValue.UpdateTime {
				// && strings.Trim(userParamValue.Value, "'") != strings.Trim(runningParamValue.Value, "'")
				if latestFlushTimeStr != "" && latestFlushTimeStr > userParamValue.UpdateTime {
					// 该参数已经执行过刷参，刷参值没变，导致running updateTime没变
					result[paramKey] = userParamValue.Value
					continue
				}
				changed = true
				currentNeedRestart := paramNeedRestart(getTemplateParam(paramKey, templateParams))
				if currentNeedRestart {
					needRestart = true
				}
				result[paramKey] = userParamValue.Value
				changedParams[paramKey] = ChangedParamItemValue{
					OldValue:    oldValue,
					NewValue:    userParamValue.Value,
					NeedRestart: currentNeedRestart,
				}
				continue
			}
			// 用户没有更改过，也没有未生效参数，返回的running值要检查是否需要加上引号
			// 虽加引号，但是值没变，不需要重启
			if !inoperative || (inoperative && runningParamValue.Inoperative == "") {
				regex := ""
				item := getTemplateParam(paramKey, templateParams)
				if item != nil {
					regex = item.Optional
				}
				result[paramKey] = getRunningParamQuotaValue(userParamValue.Value, runningParamValue.Value, regex)
				continue
			}
		}
		if inoperative && userParams != nil && runningParamValue.Inoperative != "" {
			// 用户没有更改过，但running中包含未生效参数
			changed = true
			currentNeedRestart := paramNeedRestart(getTemplateParam(paramKey, templateParams))
			if currentNeedRestart {
				needRestart = true
			}
			regex := ""
			item := getTemplateParam(paramKey, templateParams)
			if item != nil {
				regex = item.Optional
			}
			result[paramKey] = getRunningParamQuotaValue(userParamValue.Value, runningParamValue.Inoperative, regex)
			changedParams[paramKey] = ChangedParamItemValue{
				OldValue:    oldValue,
				NewValue:    result[paramKey],
				NeedRestart: currentNeedRestart,
			}
			continue
		}
		// user里面不包含，running包含的，不刷参
	}

	for _, paramItem := range templateParams {
		// 参数模板中包含，running中没有，是不生效参数
		if _, ok := result[paramItem.Name]; !ok {
			continue
		}
		// 参数模板中不予许修改的值，并且不是ssl相关参数，做覆盖
		needOverride := !canUpdateNotChangeableParams &&
			paramItem.IsUserChangeable == 0 &&
			paramItem.IsDeleted == 0 &&
			result[paramItem.Name] != paramItem.DefaultValue &&
			!utils.ContainsString(define.IgnoreTemplateParams, paramItem.Name, nil)

		if needOverride {
			changed = true
			if paramItem.IsDynamic == 0 {
				needRestart = true
			}
			result[paramItem.Name] = paramItem.DefaultValue
			changedParams[paramItem.Name] = ChangedParamItemValue{
				OldValue:    result[paramItem.Name],
				NewValue:    paramItem.DefaultValue,
				NeedRestart: paramItem.IsDynamic == 0,
			}
		}
	}

	for paramKey, userParamValue := range userParams {
		if utils.ContainsString(define.IgnoreParams, paramKey, nil) {
			continue
		}
		if _, resultContainsKey := result[paramKey]; !resultContainsKey {
			// 结果集里面有不包含的user值，使用户值生效
			changed = true
			currentNeedRestart := paramNeedRestart(getTemplateParam(paramKey, templateParams))
			if currentNeedRestart {
				needRestart = true
			}
			result[paramKey] = userParamValue.Value
			changedParams[paramKey] = ChangedParamItemValue{
				OldValue:    "",
				NewValue:    userParamValue.Value,
				NeedRestart: currentNeedRestart,
			}
		}
	}
	dbCluster.Logger.Info("GetEffectiveParams succeed.", "changed", changed, "needRestart", needRestart, "changedParams", changedParams)

	return result, changed, needRestart, nil
}

// 当变配时，UserParams要和RunningParams一致，否则管控无法判断以谁为准，要做前置校验。
// resetClassChangeableValue 如果前端提示用户规格参数和用户设置的有冲突，console负责把用户解决的冲突写回UserParams，这里设置为false
// 变配执行时，当前UserParams要合并参数模板中不可变部分和规格默认参数
func (dbCluster *DbClusterBase) GetEffectiveUserParams(resetClassChangeableValue bool, canUpdateNotChangeableParams bool) (map[string]*EffectiveUserParamValue, bool, bool, error) {
	var oldClassKey = ""
	if dbCluster.OldClassInfo != nil {
		oldClassKey = dbCluster.OldClassInfo.ClassName
	}
	newClassKey := dbCluster.ClassInfo.ClassName
	engineType := dbCluster.EngineType
	changed := false
	needRestart := false
	result := make(map[string]*EffectiveUserParamValue)

	userParams, _, err := dbCluster.EngineParamsRepo.GetUserParams(dbCluster)
	if err != nil {
		return nil, false, false, err
	}

	newClassParams, err := dbCluster.ParamsClassQuery.GetClassParams(engineType, newClassKey)
	if err != nil {
		return nil, false, false, err
	}

	runningParams, err := dbCluster.EngineParamsRepo.GetRunningParams(dbCluster)
	if err != nil {
		return nil, false, false, err
	}

	for paramKey, userParamValue := range userParams {
		result[paramKey] = &EffectiveUserParamValue{
			userParamValue.Value,
			true,
			"",
		}
	}

	paramsTemplate, err := NewParamsTemplate(dbCluster.EngineType, dbCluster.ClassInfo.ClassName, dbCluster.ParamsTemplateQuery, dbCluster.ParamsClassQuery, dbCluster.Logger)
	if err != nil {
		return nil, false, false, err
	}
	templateParams := paramsTemplate.getClassParamsTemplate()

	var oldClassParams map[string]string

	for _, paramItem := range templateParams {
		var (
			itemResultValue   string
			runningUpdateTime string
			needFlush         = true
		)

		if result[paramItem.Name] != nil {
			itemResultValue = result[paramItem.Name].Value
		}
		// 参数模板中不予许修改的值，做覆盖
		needOverride1 := !canUpdateNotChangeableParams &&
			paramItem.IsUserChangeable == 0 &&
			paramItem.IsDeleted == 0 &&
			itemResultValue != paramItem.DefaultValue &&
			!utils.ContainsString(define.IgnoreTemplateParams, paramItem.Name, nil)

		// 包含在规格参数中，参数模板中允许更改，并且指定需要覆盖
		newClassParamValue, isClassParam := newClassParams[paramItem.Name]
		needOverride2 := isClassParam &&
			resetClassChangeableValue &&
			paramItem.IsUserChangeable == 1 &&
			itemResultValue != newClassParamValue

		// 包含在规格参数中，参数模板中允许更改，并且用户未更改过原规格默认值，无论是否指定覆盖，都进行覆盖 （变配场景）
		needOverride3 := false
		if oldClassKey != newClassKey && newClassKey != "" && oldClassKey != "" {
			if oldClassParams == nil {
				oldClassParams, err = dbCluster.ParamsClassQuery.GetClassParams(engineType, oldClassKey)
				if err != nil {
					return nil, false, false, err
				}
			}
			oldClassParamValue, isClassParam := oldClassParams[paramItem.Name]
			needOverride3 = isClassParam &&
				paramItem.IsUserChangeable == 1 &&
				itemResultValue == oldClassParamValue &&
				itemResultValue != newClassParamValue
		}

		if needOverride1 || needOverride2 || needOverride3 {
			if runningParamValue, ok := runningParams[paramItem.Name]; ok && paramItem.DefaultValue == runningParamValue.Value {
				// running和模板值一致，只订正，无需刷参
				needFlush = false
				runningUpdateTime = runningParamValue.UpdateTime
			}
			changed = true
			if paramItem.IsDynamic == 0 && needFlush {
				needRestart = true
			}
			result[paramItem.Name] = &EffectiveUserParamValue{
				paramItem.DefaultValue,
				needFlush,
				runningUpdateTime,
			}
			dbCluster.Logger.Info(fmt.Sprintf("GetEffectiveUserParams need override [%s] [%+v]", paramItem.Name, result[paramItem.Name]))
		}
	}

	return result, changed, needRestart, nil
}

// 获取用户设置的参数值
func (dbCluster *DbClusterBase) GetUserParams() (map[string]*ParamItem, string, error) {
	return dbCluster.EngineParamsRepo.GetUserParams(dbCluster)
}

// 获取运行中的参数值
func (dbCluster *DbClusterBase) GetRunningParams() (map[string]*ParamItem, error) {
	return dbCluster.EngineParamsRepo.GetRunningParams(dbCluster)
}

func templateContainsParam(key string, templateParams []*ParamTemplateItem) bool {
	for _, paramItem := range templateParams {
		if key == paramItem.Name {
			return true
		}
	}
	return false
}

func paramNeedRestart(templateParam *ParamTemplateItem) bool {
	if templateParam != nil {
		return templateParam.IsDynamic == 0
	}
	// 不在参数模板中的参数，都要重启
	return true
}

func getTemplateParam(paramName string, templateParams []*ParamTemplateItem) *ParamTemplateItem {
	for _, paramItem := range templateParams {
		if paramItem.Name == paramName {
			return paramItem
		}
	}
	return nil
}

func getRunningParamQuotaValue(userParamValue, runningParamValue, regex string) string {
	quotaMark := `^'.*'$`
	regQuotaMark := `^\^'.*'\$$`
	regSplitMark := `[,:]`
	isUserQuota, err := regexp.Match(quotaMark, []byte(userParamValue))
	if err != nil {
		return runningParamValue
	}
	isRunningQuota, err := regexp.Match(quotaMark, []byte(runningParamValue))
	if err != nil {
		return runningParamValue
	}
	isRegQuota := false
	if regex != "" {
		isRegQuota, err = regexp.Match(regQuotaMark, []byte(regex))
		if err != nil {
			return runningParamValue
		}
	}
	valueContainSplit := false
	valueContainSplit, err = regexp.Match(regSplitMark, []byte(runningParamValue))
	if err != nil {
		return runningParamValue
	}
	if (isUserQuota || isRegQuota || valueContainSplit) && !isRunningQuota {
		return fmt.Sprintf("'%s'", runningParamValue)
	} else {
		return runningParamValue
	}
}

func GetCanUpdateNotChangeableParams(logger logr.Logger) bool {
	result := false

	if !result {
		controllerConf, err := k8sutil.GetControllerConfig(logger)
		if err != nil {
			logger.Error(err, "GetControllerConfig error")
		} else {
			result = controllerConf.CanUpdateNotChangeableParams
		}
	}
	return result
}
