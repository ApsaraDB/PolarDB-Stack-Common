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


package define

import corev1 "k8s.io/api/core/v1"

// 选主全局开关
var (
	IsLeader = false
)

const (
	UnixTimeTemplate                                            = "2006-01-02T15:04:05Z"
	ParamsTemplateNamespace                                     = "kube-system"
	MinorVersionCmNamespace                                     = "kube-system"
	UserParamsSuffix                                            = "-user-params"
	RunningParamsSuffix                                         = "-running-params"
	PortUsageCmName                                             = "cloud-provider-port-usage-%s"
	PortUsageCmNamespace                                        = "kube-system"
	NextExternalPortKey                                         = "Pbx.Next.External.Port"
	PortRangeCmNamespace                                        = "kube-system"
	InsIdRangeCmNamespace                                       = "kube-system"
	NextClusterScopeCustInsIdKey                                = "NextClusterScopeCustInsId"
	DefaultNextClusterScopeCustInsId                            = "1000"
	StorageDummyWorkflowId                                      = "Dummy#000"
	ContainerNameEngine                                         = "engine"
	ContainerNamePfsd                                           = "pfsd"
	ContainerNamePfsdTool                                       = "pfsd-tool"
	InstanceSystemResourcesCmName                               = "instance-system-resources"
	InstanceSystemResourcesCmNamespace                          = "kube-system"
	ClusterManagerNameSuffix                                    = ".cm"
	ClusterManagerFlagSuffix                                    = "-cmflag"
	ClusterManagerMetaSuffix                                    = "-cmconfig"
	ClusterManagerLeaderSuffix                                  = "-cmleader"
	ClusterManagerGlobalConfigSuffix                            = "-global-cmconfig"
	CmHASwitchFlagKey                                           = "enable_all"
	CmEnableOnlinePromoteFlagKey                                = "enable_online_promote"
	CmEnableRedlineKey                                          = "enable_redline"
	CmHASwitchFlagKeyEnable                                     = "1"
	StorageConditionType               corev1.NodeConditionType = "PlugInStorageUnavailable"
	ClientNetworkConditionType         corev1.NodeConditionType = "NodeClientNetworkUnavailable"

	// Manager
	SrvOperatorPrefix                      = "srv_opr"
	SrvOperatorAction                      = SrvOperatorPrefix + "_action"
	SrvOperatorType                        = SrvOperatorPrefix + "_type"
	SrvOperatorHostinsHosts                = SrvOperatorPrefix + "_hosts"
	SrvOperatorHostIp                      = SrvOperatorPrefix + "_host_ip"
	SrvOperatorUserName                    = SrvOperatorPrefix + "_user_name"
	SrvOperatorUserPassword                = SrvOperatorPrefix + "_password"
	SrvOperatorUserPrivilege               = SrvOperatorPrefix + "_privilege"
	ManagerSrvTypeIns                      = "hostins_ops"
	ManagerSrvTypeAccount                  = "account"
	ManagerSrvTypeHealthCheck              = "health_check"
	ManagerSrvTypeLockIns                  = "lock_ins"
	ManagerSrvTypeUpdateConf               = "update_conf"
	ManagerSrvTypeReplica                  = "replica"
	ManagerActionCreateReplication         = "create_replication"
	ManagerActionSetupInstall              = "setup_install_instance"
	ManagerActionSetupLogAgent             = "setup_logagent_config"
	ManagerActionGetSystemIdentifier       = "get_system_identifier"
	ManagerActionQueryInstallationProgress = "query_installation_progress"
	ManagerActionDisableSsl                = "disable_ssl"
	ManagerActionStopInstance              = "stop_instance"
	ManagerActionStartInstance             = "start_instance"
	ManagerActionGrowPfs                   = "grow_pfs"
	ManagerActionUpdateConf                = "update"
	ManagerActionCreateAccount             = "create"
	ManagerActionHealthCheck               = "hostins_check"
	ManagerActionLockDiskFull              = "lock_ins_diskfull"
	ManagerActionUnLockDiskFull            = "unlock_ins_diskfull"
	NetConfigMapNamespace                  = "kube-system"
	NetConfigmapName                       = "ccm-config"
	DefaultClientNetCardName               = "bond1"
	InternalNetCardName                    = "INTERNAL_NETWORK_CARD_NAME"
	ClientNetCardName                      = "NET_CARD_NAME"

	StorageServiceNamespace = "kube-system"
	StorageServiceName      = "polardb-sms-manager"

	// Cache
	CacheKeyNodesClientIP = "CacheKeyNodesClientIP"

	EngineImageName         = "engineImage"
	ManagerImageName        = "managerImage"
	PfsdImageName           = "pfsdImage"
	PfsdToolImageName       = "pfsdToolImage"
	ClusterManagerImageName = "clusterManagerImage"

	EnginePhasePending  string = "PENDING"
	EnginePhaseStarting string = "STARTING"
	EnginePhaseRunning  string = "RUNNING"
	EnginePhaseStopping string = "STOPPING"
	EnginePhaseStopped  string = "STOPPED"
	EnginePhaseFailed   string = "FAILED"

	LabelInsType           string = "apsara.metric.db_ins_type"
	LabelClusterName       string = "apsara.metric.clusterName"
	LabelClusterMode       string = "apsara.ins.cluster.mode"
	LabelInsPort           string = "apsara.ins.port"
	LabelArch              string = "apsara.metric.arch"
	LabelDbType            string = "apsara.metric.db_type"
	LabelInsId             string = "apsara.metric.ins_id"
	LabelInsName           string = "apsara.metric.ins_name"
	LabelLogicCustInsId    string = "apsara.metric.logic_custins_id"
	LabelLogicCustInsName  string = "apsara.metric.logic_custins_name"
	LabelPhysicalCustInsId string = "apsara.metric.physical_custins_id"
	LabelPodName           string = "apsara.metric.pod_name"
	LabelPvName            string = "apsara.metric.pv_name"
)

// 忽略以下模板参数的覆盖
var IgnoreTemplateParams []string = []string{"ssl", "ssl_cert_file", "ssl_key_file", "listen_addresses"}

// 忽略以下参数的更新
var IgnoreParams []string = []string{"log_line_prefix"}
