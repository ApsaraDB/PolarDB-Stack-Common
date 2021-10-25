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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ApsaraDB/PolarDB-Stack-Common/configuration"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"

	"github.com/go-logr/logr"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils"

	"k8s.io/apimachinery/pkg/types"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	"github.com/ApsaraDB/PolarDB-Stack-Common/define"
	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

type EnvGetStrategyBase struct {
	IEnvGetStrategy
	GetClusterName       func() string
	GetFlushEnvConfigMap func() (*v1.ConfigMap, error)
	AccountRepository    domain.IAccountRepository
	Ins                  *domain.DbIns
	Logger               logr.Logger
}

func (d *EnvGetStrategyBase) GetPodName() (string, string) {
	return d.Ins.ResourceName, d.Ins.ResourceNamespace
}

func (d *EnvGetStrategyBase) GetStartEngineEnvirons() (string, error) {
	return d.GetCommonEnv(define.ManagerSrvTypeIns, define.ManagerActionStartInstance, false)
}

func (d *EnvGetStrategyBase) GetSetupLogAgentEnvirons() (string, error) {
	podName, podNamespace := d.GetPodName()
	pod, err := k8sutil.GetPod(podName, podNamespace, d.Logger)
	if err != nil {
		return "", err
	}
	containerId := ""
	for _, container := range pod.Status.ContainerStatuses {
		if container.Name == define.ContainerNameEngine {
			containerId = container.ContainerID
		}
	}
	containerId = strings.Replace(containerId, "docker://", "", 1)
	if containerId == "" {
		return "", errors.Errorf("engine container id is illegal")
	}

	envs := map[string]string{
		define.SrvOperatorType:   define.ManagerSrvTypeIns,
		define.SrvOperatorAction: define.ManagerActionSetupLogAgent,
		"base_collect_path":      "/sys/fs/cgroup/",
		"ins_name":               d.Ins.InsName,
		"pod_collect_path":       "/kubepods/burstable/pod" + string(pod.UID) + "/" + containerId,
		"access_port":            strconv.Itoa(d.Ins.NetInfo.Port),
		"phy_ins_id":             d.Ins.PhysicalInsId,
	}

	return d.TranslateEnvMap2String(envs), nil
}

func (d *EnvGetStrategyBase) GetGracefulStopEngineEnvirons() (string, error) {
	return d.GetCommonEnv(define.ManagerSrvTypeIns, define.ManagerActionStopInstance, false)
}

func (d *EnvGetStrategyBase) GetUnLockEngineEnvirons() (string, error) {
	return d.GetCommonEnv(define.ManagerSrvTypeLockIns, define.ManagerActionUnLockDiskFull, false, define.SrvOperatorHostIp)
}

func (d *EnvGetStrategyBase) GetLockEngineEnvirons() (string, error) {
	return d.GetCommonEnv(define.ManagerSrvTypeLockIns, define.ManagerActionLockDiskFull, false, define.SrvOperatorHostIp)
}

func (d *EnvGetStrategyBase) GetGrowStorageEnvirons() (string, error) {
	return d.GetCommonEnv(define.ManagerSrvTypeIns, define.ManagerActionGrowPfs, true, define.SrvOperatorHostIp)
}

func (d *EnvGetStrategyBase) GetPrimarySystemIdentifier() (string, error) {
	return d.GetCommonEnv(define.ManagerSrvTypeIns, define.ManagerActionGetSystemIdentifier, true)
}

func (d *EnvGetStrategyBase) GetCreateEngineAccountEnvirons() ([]string, error) {
	baseEnv, err := d.GetCommonEnv(define.ManagerSrvTypeAccount, define.ManagerActionCreateAccount, false, define.SrvOperatorHostIp)
	if err != nil {
		return nil, err
	}
	accounts, err := d.GetAccountsFromMeta()
	if err != nil {
		return nil, err
	}
	var envArrays []string = nil
	envs := map[string]string{}
	for _, account := range accounts {
		envs[define.SrvOperatorUserName] = account.Account
		envs[define.SrvOperatorUserPassword] = account.Password
		envs[define.SrvOperatorUserPrivilege] = strconv.Itoa(account.PriviledgeType)
		stringEnvs := d.TranslateEnvMap2StringWithBase(envs, baseEnv)
		envArrays = append(envArrays, stringEnvs)
	}
	return envArrays, nil
}

func (d *EnvGetStrategyBase) GetInstallationProgressEnvirons() (string, error) {
	return d.GetCommonEnv(define.ManagerSrvTypeIns, define.ManagerActionQueryInstallationProgress, true)
}

func (d *EnvGetStrategyBase) GetCreateReplicationSlotEnvirons(roResourceName string) (string, error) {
	baseEnv, err := d.GetCommonEnv(
		define.ManagerSrvTypeReplica,
		define.ManagerActionCreateReplication,
		false,
		"service_type")

	port := d.Ins.NetInfo.Port
	rwPort := &Port{
		Link:       []int{port},
		AccessPort: []int{port},
		PerfPort:   []int{port},
	}
	strRwPort, err := rwPort.ToString()
	if err != nil {
		return "", err
	}

	rwUniqueId, err := d.GetUniqueId(d.Ins.ResourceName)
	if err != nil {
		return "", err
	}

	roPhyId, roUniqueId, err := d.GetPhysicalAndInsId(roResourceName)
	if err != nil {
		return "", err
	}

	rwInsInfoStr, err := d.getInsInfo(d.Ins.ResourceName, d.Ins.PhysicalInsId, rwUniqueId, err)
	if err != nil {
		return "", err
	}

	roInsInfoStr, err := d.getInsInfo(roResourceName, roPhyId, roUniqueId, err)
	if err != nil {
		return "", err
	}

	envs := map[string]string{}
	envs["current_ins"] = rwInsInfoStr
	envs["ro_custins_current"] = roInsInfoStr
	envs["pod_name"] = roResourceName
	envs["slot_unique_name"] = roUniqueId
	envs["srv_opr_host_ip"] = strRwPort

	return d.TranslateEnvMap2StringWithBase(envs, baseEnv), nil
}

func (d *EnvGetStrategyBase) getInsInfo(resourceName, phyId, uniqueId string, err error) (string, error) {
	insInfo := map[string]string{
		"pod_name":         resourceName,
		"custins_id":       phyId,
		"slot_unique_name": uniqueId,
	}
	insInfoStr, err := json.Marshal(insInfo)
	if err != nil {
		return "", err
	}
	return string(insInfoStr), nil
}

func (d *EnvGetStrategyBase) GetUpdateEngineParamsEnvirons(engineParams map[string]string) (string, error) {
	envs, err := d.GetCommonEnv(define.ManagerSrvTypeUpdateConf, define.ManagerActionUpdateConf, false, define.SrvOperatorHostIp, "cluster_custins_info", "cust_ins_id", "ins_id")
	if err != nil {
		return "", err
	}
	byteParams, err := json.Marshal(engineParams)
	if err != nil {
		return "", errors.Wrap(err, "marshal engineParams error")
	}
	strParams := string(byteParams)
	strParams = strings.Replace(strParams, "\\", "\\\\", -1)
	strParams = strings.Replace(strParams, "\"", "\\\"", -1)
	envs += "params=\"" + strParams + "\" "

	return envs, nil
}

func (d *EnvGetStrategyBase) GetCheckHealthEnvirons() (string, error) {
	return d.GetCommonEnv(define.ManagerSrvTypeHealthCheck, define.ManagerActionHealthCheck, false, define.SrvOperatorHostIp)
}

func (d *EnvGetStrategyBase) GetEnvConfigMap() (*v1.ConfigMap, error) {
	foundPodConfigmap := &v1.ConfigMap{}
	err := mgr.GetSyncClient().Get(context.TODO(), types.NamespacedName{Namespace: d.Ins.ResourceNamespace, Name: d.Ins.ResourceName}, foundPodConfigmap)
	return foundPodConfigmap, err
}

func (d EnvGetStrategyBase) GetCommonEnv(operatorType string, operatorAction string, allFields bool, fields ...string) (string, error) {
	var needUpdateCm bool
	if allFields || len(fields) > 0 {
		needUpdateCm = true
	}
	envs := map[string]string{
		define.SrvOperatorType:   operatorType,
		define.SrvOperatorAction: operatorAction,
	}
	if needUpdateCm {
		var foundConfigMap *v1.ConfigMap
		var err error
		if foundConfigMap, err = d.GetFlushEnvConfigMap(); err != nil {
			return "", errors.Wrap(err, "getFlushEnvConfigMap error")
		}

		if utils.ContainsString(fields, define.SrvOperatorHostIp, nil) {
			portInfo := foundConfigMap.Data["port"]
			var portData map[string]interface{}
			err = json.Unmarshal([]byte(portInfo), &portData)
			if err != nil {
				return "", err
			}
			hostInsId := foundConfigMap.Data["ins_id"]
			hosts, _ := json.Marshal(portData[hostInsId])
			envs[define.SrvOperatorHostIp] = string(hosts)
		}
		// envs[define.SrvOperatorHostinsHosts] = "[" + string(hosts) + "]"
		if allFields {
			for key, val := range foundConfigMap.Data {
				envs[key] = val
			}
		} else {
			for _, field := range fields {
				if field == define.SrvOperatorHostIp {
					continue
				}
				var done bool
				for key, val := range foundConfigMap.Data {
					if key != field {
						continue
					}
					envs[key] = val
					done = true
					break
				}
				if !done {
					err := errors.New(fmt.Sprintf("param %s can not found", field))
					d.Logger.Error(err, "")
					return "", err
				}
			}
		}
	}
	stringEnvs := d.TranslateEnvMap2String(envs)
	return stringEnvs, nil
}

func (d EnvGetStrategyBase) TranslateEnvMap2String(envMap map[string]string) string {
	return d.TranslateEnvMap2StringWithBase(envMap, "")
}

func (d EnvGetStrategyBase) TranslateEnvMap2StringWithBase(envMap map[string]string, baseString string) string {
	for key, value := range envMap {
		if key == "mycnf_dict" {
			// 由于mycnf_dict中存在 ' 特殊字符，所以不能使用 'value' 来传递给内核中, 所以修改使用"value" 来传递。
			value = strings.Replace(value, "\"", "\\\"", -1)
			baseString += key + "=\"" + value + "\" "
		} else {
			baseString += key + "='" + value + "' "
		}
		d.Logger.V(8).Info(fmt.Sprintf("k=%v, v=%v\n", key, value))
	}
	return baseString
}

func (d EnvGetStrategyBase) GetUniqueId(resourceName string) (string, error) {
	uniqueId := strings.Split(resourceName, "-")[len(strings.Split(resourceName, "-"))-1]
	if uniqueId == "" {
		return "", errors.New("unique id can not be nil")
	}
	return uniqueId, nil
}

func (d EnvGetStrategyBase) GetPhysicalAndInsId(resourceName string) (string, string, error) {
	ids := strings.Split(resourceName, "-")
	idsLength := len(ids)
	physicalId := ids[idsLength-2]
	if physicalId == "" {
		return "", "", errors.New("physical id can not be nil")
	}
	insId := ids[idsLength-1]
	if insId == "" {
		return "", "", errors.New("ins id can not be nil")
	}
	return physicalId, insId, nil
}

func (d EnvGetStrategyBase) GetEnvPort() (*EnvPort, error) {
	envPort := EnvPort{}
	port := d.Ins.NetInfo.Port
	insId := d.Ins.InsId
	envPort[insId] = &Port{
		Link:       []int{port},
		AccessPort: []int{port},
		PerfPort:   []int{port},
	}
	return &envPort, nil
}

func (d EnvGetStrategyBase) GetAccountsFromMeta() (accounts map[string]*Account, err error) {
	accMap, err := d.AccountRepository.GetAccounts(d.GetClusterName(), d.Ins.ResourceNamespace)
	if err != nil {
		return nil, err
	}
	var result = make(map[string]*Account)
	for key, account := range accMap {
		result[key] = &Account{
			PriviledgeType: account.PrivilegedType,
			Account:        account.Account,
			Password:       account.Password,
		}
	}
	return result, nil
}

func (d EnvGetStrategyBase) GetPhysicalCustInstanceEnv(accounts map[string]*Account, insId, hostIP string, port int, pbd *Pbd, role string, insType int) (physicalCustInstance *PhysicalCustInstance) {
	return &PhysicalCustInstance{
		Accounts: accounts,
		PbdList:  []*Pbd{pbd},
		HostIns: map[string]*HostIns{
			insId: {
				Role:  role,
				Port:  port,
				InsIp: hostIP,
			},
		},
		InsType: insType,
	}
}

type Account struct {
	PriviledgeType int    `json:"priviledge_type"`
	Account        string `json:"account"`
	Password       string `json:"password"`
}

type Pbd struct {
	PbdNumber   int64  `json:"pbd_number"`
	PbdName     string `json:"pbd_name"`
	Label       string `json:"label"`
	DataVersion int64  `json:"data_version"`
	CustinsId   int    `json:"custins_id"`
	EngineType  string `json:"engine_type"`
}

type PhysicalCustInstance struct {
	Accounts map[string]*Account `json:"accounts"`
	PbdList  []*Pbd              `json:"pbd_list"`
	HostIns  map[string]*HostIns `json:"hostins"`
	InsType  int                 `json:"ins_type"`
}

type HostIns struct {
	Role  string `json:"role"`
	Port  int    `json:"port"`
	InsIp string `json:"ins_ip"`
}

type EnvClusterInfo struct {
	DataMaxClusterInfo map[string]*PhysicalCustInstance `json:"cluster_info_datamax_placeholder,omitempty"`
	RwClusterInfo      map[string]*PhysicalCustInstance `json:"cluster_info_rw_placeholder,omitempty"`
	RoClusterInfo      map[string]*PhysicalCustInstance `json:"cluster_info_ro_placeholder,omitempty"`
	StandbyClusterInfo map[string]*PhysicalCustInstance `json:"cluster_info_standby_placeholder,omitempty"`
}

func (r *EnvClusterInfo) ToString() (string, error) {
	str, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	result := string(str)
	result = strings.Replace(result, "cluster_info_rw_placeholder", configuration.GetConfig().ClusterInfoDbInsTypeRw, -1)
	result = strings.Replace(result, "cluster_info_ro_placeholder", configuration.GetConfig().ClusterInfoDbInsTypeRo, -1)
	result = strings.Replace(result, "cluster_info_datamax_placeholder", configuration.GetConfig().ClusterInfoDbInsTypeDataMax, -1)
	result = strings.Replace(result, "cluster_info_standby_placeholder", configuration.GetConfig().ClusterInfoDbInsTypeStandby, -1)
	return result, nil
}

type Port struct {
	Link       []int `json:"link"`
	AccessPort []int `json:"access_port"`
	PerfPort   []int `json:"perf_port"`
}

func (r *Port) ToString() (string, error) {
	str, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(str), nil
}

type EnvPort map[string]*Port

func (r *EnvPort) ToString() (string, error) {
	str, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(str), nil
}
