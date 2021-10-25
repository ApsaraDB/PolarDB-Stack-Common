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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"os"
	"strconv"
	"strings"

	"github.com/ApsaraDB/PolarDB-Stack-Common/configuration"
	"github.com/ApsaraDB/PolarDB-Stack-Common/manager"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	"github.com/ApsaraDB/PolarDB-Stack-Common/define"
	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var podDeadLineSecond = int32(20)

func NewClusterManagerCreator(logger logr.Logger, accountRepository domain.IAccountRepository) *ClusterManagerCreator {
	return &ClusterManagerCreator{
		logger:            logger.WithValues("component", "ClusterManagerCreator"),
		accountRepository: accountRepository,
	}
}

type ClusterManagerCreator struct {
	RwID                string
	LogicInsId          string
	ClusterManagerImage string
	Port                int
	ConsensusPort       int
	WorkModel           domain.CmWorkMode
	logger              logr.Logger
	accountRepository   domain.IAccountRepository
}

func (m *ClusterManagerCreator) CreateClusterManager(ctx context.Context, kubeObj metav1.Object, workMode domain.CmWorkMode, logicInsId, rwPhyId, cmImage string, port int, pluginConf map[string]interface{}, consensusPort int) error {
	m.RwID = rwPhyId
	m.LogicInsId = logicInsId
	m.ClusterManagerImage = cmImage
	m.Port = port
	m.ConsensusPort = consensusPort
	m.WorkModel = workMode
	m.logger = m.logger.WithValues("namespace", kubeObj.GetNamespace(), "name", kubeObj.GetName())

	// 1. grant privilege for ClusterManager
	if err := m.ensureClusterManagerRBAC(ctx, kubeObj.GetNamespace()); err != nil {
		return errors.Wrap(err, "ensureClusterManagerRBAC error")
	}

	// 2. create config map for initialize env
	if err := m.ensureClusterManagerConfigMapUptoDate(ctx, kubeObj, pluginConf, consensusPort); err != nil {
		return errors.Wrap(err, "ensureClusterManagerConfigMapUptoDate error")
	}

	// 3. create config map for ClusterManager store meta
	if err := m.ensureClusterManagerMetaConfigUptoDate(ctx, kubeObj); err != nil {
		return errors.Wrap(err, "ensureClusterManagerMetaConfigUptoDate error")
	}

	// 4. create flag config map for ClusterManager change flag online
	if err := m.ensureClusterManagerFlagConfigUptoDate(ctx, m.logger, kubeObj); err != nil {
		return errors.Wrap(err, "ensureClusterManagerFlagConfigUptoDate error")
	}

	// 5. create cluster manager resources
	clusterManagerDeployment, err := m.getClusterManagerDeployment(ctx, m.logger, kubeObj, configuration.GetConfig().ImagePullPolicy)
	if err != nil {
		return errors.Wrap(err, "Failed to acquire clusterManagerDeployment")
	}

	clusterManagerName := kubeObj.GetName() + define.ClusterManagerNameSuffix
	found := &v1.Deployment{}
	err = mgr.GetSyncClient().Get(context.TODO(), types.NamespacedName{Name: clusterManagerName, Namespace: kubeObj.GetNamespace()}, found)
	if err != nil && apierrors.IsNotFound(err) {
		err = mgr.GetSyncClient().Create(context.TODO(), clusterManagerDeployment)
		if err != nil {
			return errors.Wrap(err, "Failed to create clusterManagerDeployment")
		}
	} else if err != nil {
		return errors.Wrap(err, "Failed to Get clusterManagerDeployment")
	}

	return nil
}

func (m *ClusterManagerCreator) ensureClusterManagerRBAC(ctx context.Context, namespace string) error {
	m.logger.Info("Start ensureClusterManagerRBAC.")
	found := &rbacv1.ClusterRoleBinding{}
	err := mgr.GetSyncClient().Get(context.TODO(), types.NamespacedName{
		Name: namespace + "-cluster-manager-rolebinding",
	}, found)
	if err != nil && apierrors.IsNotFound(err) {
		roleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace + "-cluster-manager-rolebinding",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     configuration.GetConfig().ControllerManagerRoleName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "default",
					Namespace: namespace,
				},
			},
		}
		err = mgr.GetSyncClient().Create(context.TODO(), roleBinding)
		if err != nil {
			return errors.Wrap(err, "Failed to create cluster manager rolebinding")
		}
	} else if err != nil {
		return errors.Wrap(err, "Failed to get cluster manager rolebinding")
	}

	return nil
}

func (m *ClusterManagerCreator) ensureClusterManagerConfigMapUptoDate(ctx context.Context, kubeObj metav1.Object, pluginConf map[string]interface{}, consensusPort int) error {
	type AccountInfo struct {
		AuroraUser      string `json:"aurora_user"`
		AuroraPassword  string `json:"aurora_password"`
		ReplicaUser     string `json:"replica_user"`
		ReplicaPassword string `json:"replica_password"`
	}
	type ClusterInfo struct {
		Namespace      string `json:"namespace"`
		Name           string `json:"name"`
		LogicID        string `json:"logic_id"`
		InitRwID       string `json:"init_rw_id"`
		Port           int    `json:"port"`
		PrimaryService string `json:"primary_service_name"`
	}

	clusterName := kubeObj.GetName()
	clusterManagerConfigMapName := clusterName + define.ClusterManagerNameSuffix

	accounts, err := m.accountRepository.GetAccounts(clusterName, kubeObj.GetNamespace())
	if err != nil {
		return err
	}
	var (
		auroraAccount     = accounts["aurora"]
		replicatorAccount = accounts["replicator"]
	)
	if auroraAccount == nil || replicatorAccount == nil {
		return errors.New("account is not exists")
	}

	accountInfo := &AccountInfo{
		AuroraUser:      auroraAccount.Account,
		AuroraPassword:  auroraAccount.Password,
		ReplicaUser:     replicatorAccount.Account,
		ReplicaPassword: replicatorAccount.Password,
	}

	var data = map[string]string{}
	if m.WorkModel != domain.CmWorkModePure {
		clusterInfo := &ClusterInfo{
			Namespace:      kubeObj.GetNamespace(),
			Name:           kubeObj.GetName(),
			LogicID:        m.LogicInsId,
			InitRwID:       m.RwID,
			Port:           m.Port,
			PrimaryService: clusterName + "-primaryendpoint-noclusterip",
		}

		clusterInfoJson, err := json.Marshal(clusterInfo)
		if err != nil {
			return err
		}
		accountInfoJson, err := json.Marshal(accountInfo)
		if err != nil {
			return err
		}

		data = map[string]string{
			"cluster_info": string(clusterInfoJson),
			"account_info": string(accountInfoJson),
		}
	} else {
		conf := map[string]interface{}{
			"consensus": map[string]interface{}{
				"port":         m.ConsensusPort,
				"etcd_storage": true,
			},
			"work_mode": string(m.WorkModel),
			"cluster_info": map[string]interface{}{
				"namespace": kubeObj.GetNamespace(),
				"name":      kubeObj.GetName(),
				"port":      m.Port,
			},
			"account_info": accountInfo,
			"node_driver_info": map[string]int{
				"port": 22345,
			},
		}
		confJson, err := json.Marshal(conf)
		if err != nil {
			return err
		}
		data["polardb_cluster_manager.conf"] = string(confJson)
		if pluginConf != nil {
			pluginConfJson, err := json.Marshal(pluginConf)
			if err != nil {
				return err
			}
			data["subscribe.conf"] = string(pluginConfJson)
		}
	}

	return m.ensureClusterManagerConfigUptoDate(ctx, kubeObj, clusterManagerConfigMapName, data)
}

func (m *ClusterManagerCreator) ensureClusterManagerMetaConfigUptoDate(ctx context.Context, kubeObj metav1.Object) error {
	return m.ensureClusterManagerConfigUptoDate(ctx, kubeObj, kubeObj.GetName()+define.ClusterManagerMetaSuffix, nil)
}

func (m *ClusterManagerCreator) ensureClusterManagerFlagConfigUptoDate(ctx context.Context, logger logr.Logger, kubeObj metav1.Object) error {
	dataMap := make(map[string]string)

	enableOnlinePromote := "0"
	enableRedline := "0"
	controllerConf, err := k8sutil.GetControllerConfig(logger)
	if err != nil {
		logger.Error(err, "GetControllerConfig error")
	}
	if controllerConf != nil && controllerConf.EnableOnlinePromote {
		enableOnlinePromote = "1"
	}
	if controllerConf != nil && controllerConf.EnableRedline {
		enableRedline = "1"
	}
	dataMap[define.CmHASwitchFlagKey] = define.CmHASwitchFlagKeyEnable
	dataMap[define.CmEnableOnlinePromoteFlagKey] = enableOnlinePromote
	dataMap[define.CmEnableRedlineKey] = enableRedline
	return m.ensureClusterManagerConfigUptoDate(ctx, kubeObj, kubeObj.GetName()+define.ClusterManagerFlagSuffix, dataMap)
}

func (m *ClusterManagerCreator) ensureClusterManagerConfigUptoDate(ctx context.Context, kubeObj metav1.Object, cmName string, data map[string]string) error {
	metaConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: kubeObj.GetNamespace(),
		},
		Data: data,
	}

	m.logger.Info(fmt.Sprintf("[%s] Creating ClusterManager ConfigMap", kubeObj.GetName()))
	if err := controllerutil.SetControllerReference(kubeObj, metaConfigMap, mgr.GetManager().GetScheme()); err != nil {
		return err
	}
	foundConfigMap := &corev1.ConfigMap{}
	err := mgr.GetSyncClient().Get(context.TODO(), types.NamespacedName{Namespace: kubeObj.GetNamespace(), Name: cmName}, foundConfigMap)
	if err != nil && apierrors.IsNotFound(err) {
		// not found, create clusterManagerConfigMap
		err = mgr.GetSyncClient().Create(context.TODO(), metaConfigMap)
		if err != nil {
			return err
		}
	} else if err != nil {
		// other error
		return err
	} else {
		// FIXME-GMT check here
	}
	return nil
}

func (m *ClusterManagerCreator) getClusterManagerDeployment(ctx context.Context, logger logr.Logger, kubeObj metav1.Object, imagePullPolicy corev1.PullPolicy) (*v1.Deployment, error) {
	// 不能创建在运维模式节点
	nodeSelectors := []corev1.NodeSelectorRequirement{
		{
			Key:      configuration.GetConfig().NodeMaintainLabelName,
			Operator: corev1.NodeSelectorOpDoesNotExist,
		},
	}
	controllerConf, err := k8sutil.GetControllerConfig(logger)
	if err != nil {
		err = errors.Wrapf(err, "GetControllerConfig error")
		logger.Error(err, "")
		return nil, err
	}

	enableOnlinePromote := "false"
	if controllerConf != nil && controllerConf.EnableOnlinePromote {
		enableOnlinePromote = "true"
	}

	// 非混部模式下，只能运行在控制节点
	var hybridSelectRequirement *corev1.NodeSelectorRequirement = nil
	if !controllerConf.CmHybridDeployment && controllerConf.ControllerNodeLabel != "" {
		hybridSelectRequirement = &corev1.NodeSelectorRequirement{
			Key:      controllerConf.ControllerNodeLabel,
			Operator: corev1.NodeSelectorOpExists,
		}
	}
	if hybridSelectRequirement != nil {
		nodeSelectors = append(nodeSelectors, *hybridSelectRequirement)
	}

	namespaceName := kubeObj.GetNamespace()
	clusterManagerName := kubeObj.GetName() + define.ClusterManagerNameSuffix

	var (
		replicas      int32 = 1
		memRequest          = "200Mi"
		memLimit            = "512Mi"
		cpuRequest          = "200m"
		cpuLimit            = "500m"
		workModeValue       = string(m.WorkModel)
	)

	var config = configuration.GetConfig()
	if config.CmCpuReqLimit != "" {
		if slice := strings.Split(config.CmCpuReqLimit, "/"); len(slice) == 2 {
			cpuRequest = slice[0]
			cpuLimit = slice[1]
		}
	}
	if config.CmMemReqLimit != "" {
		if slice := strings.Split(config.CmMemReqLimit, "/"); len(slice) == 2 {
			memRequest = slice[0]
			memLimit = slice[1]
		}
	}

	env := append([]corev1.EnvVar{
		{
			Name:  "client_interface_name",
			Value: GetNetworkCardName(logger, define.ClientNetCardName),
		},
		{
			Name:  "POLAR_ENABLE_ONLINE_PROMOTE",
			Value: enableOnlinePromote,
		},
		{
			Name:  "work_mode",
			Value: workModeValue,
		},
		{
			Name:  "dma_standby",
			Value: "true",
		},
		{
			Name:  "CM_LOG_FILE_NAME",
			Value: fmt.Sprintf("/kube-log/%v.log", kubeObj.GetName()),
		},
		{
			Name:  "CM_LOG_FILE_ROTATE_BY_DAY",
			Value: "TRUE",
		},
	}, buildApiAccessEvn()...)

	internalNetCardName := GetNetworkCardName(logger, define.InternalNetCardName)
	if internalNetCardName != "" {
		env = append(env, corev1.EnvVar{
			Name:  "service_interface_name",
			Value: internalNetCardName,
		})
	}

	hostPathType := corev1.HostPathDirectoryOrCreate
	mountPropagationNone := corev1.MountPropagationNone

	var containerPorts []corev1.ContainerPort
	containerPorts = append(containerPorts, corev1.ContainerPort{
		Name:          "cm",
		ContainerPort: int32(m.Port),
	})

	cmLogPath := configuration.GetConfig().CmLogHostPath
	if workModeValue == string(domain.CmWorkModePure) {
		cmLogPath += "/" + kubeObj.GetName()
	}

	clusterManager := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterManagerName,
			Namespace: namespaceName,
		},
		Spec: v1.DeploymentSpec{
			ProgressDeadlineSeconds: &podDeadLineSecond,
			Strategy: v1.DeploymentStrategy{
				Type: v1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &v1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 3,
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 3,
					},
				},
			},
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": clusterManagerName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":  clusterManagerName,
						"port": strconv.Itoa(m.Port),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "manager",
							Image:           m.ClusterManagerImage,
							ImagePullPolicy: imagePullPolicy,
							Ports:           containerPorts,
							Env:             env,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuLimit),
									corev1.ResourceMemory: resource.MustParse(memLimit),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuRequest),
									corev1.ResourceMemory: resource.MustParse(memRequest),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:             "kube-log",
									MountPath:        configuration.GetConfig().CmLogMountPath,
									MountPropagation: &mountPropagationNone,
								},
							},
						},
					},
					HostNetwork: true,
					DNSPolicy:   corev1.DNSClusterFirstWithHostNet,

					// FIXME-GMT make this a parameter
					RestartPolicy: corev1.RestartPolicyAlways,
					Volumes: []corev1.Volume{
						{
							Name: "kube-log",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: configuration.GetConfig().CmLogHostPath + "/" + kubeObj.GetName(),
									Type: &hostPathType,
								},
							},
						},
					},
					SchedulerName: "default-scheduler",
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: nodeSelectors,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	cmConfName := "polardb-cluster-manager-conf"
	if m.WorkModel == domain.CmWorkModePure {
		clusterManager.Spec.Template.Spec.Volumes = append(clusterManager.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: cmConfName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: clusterManagerName,
					},
				},
			},
		})
		clusterManager.Spec.Template.Spec.Containers[0].VolumeMounts = append(clusterManager.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      cmConfName,
			MountPath: "/root/polardb_cluster_manager/conf",
		})
	} else {
		clusterManager.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{
			{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: clusterManagerName},
				},
			},
		}
	}

	setPodAffinity(&clusterManager.Spec.Template.Spec, kubeObj.GetName())

	if err := controllerutil.SetControllerReference(kubeObj, clusterManager, mgr.GetManager().GetScheme()); err != nil {
		return nil, err
	}
	// when rw ready, check secret and create accounts
	return clusterManager, nil
}

func buildApiAccessEvn() []corev1.EnvVar {
	apiServiceHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	apiServicePort := os.Getenv("KUBERNETES_SERVICE_PORT")
	apiServiceEndPoints := os.Getenv("KUBERNETES_SERVICE_ENDPOINTS")

	if apiServiceHost == "" && apiServicePort == "" && apiServiceEndPoints == "" {
		return []corev1.EnvVar{}
	}

	var r []corev1.EnvVar
	if apiServiceHost != "" {
		r = append(r, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_HOST",
			Value: apiServiceHost,
		})
	}
	if apiServicePort != "" {
		r = append(r, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_PORT",
			Value: apiServicePort,
		})
	}
	if apiServiceEndPoints != "" {
		r = append(r, corev1.EnvVar{
			Name:  "KUBERNETES_SERVICE_ENDPOINTS",
			Value: apiServiceEndPoints,
		})
	}

	return r
}

func setPodAffinity(podSpec *corev1.PodSpec, dbClusterName string) {
	if podSpec.Affinity == nil {
		podSpec.Affinity = &corev1.Affinity{}
	}
	podSpec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "apsara.metric.clusterName",
							Operator: metav1.LabelSelectorOpIn,
							Values: []string{
								dbClusterName,
							},
						},
						{
							Key: define.LabelInsType,
							Values: []string{
								configuration.GetConfig().ClusterInfoDbInsTypeRw,
							},
							Operator: metav1.LabelSelectorOpIn,
						},
					},
				},
				TopologyKey: "kubernetes.io/hostname", //#节点所属拓朴域
			},
		},
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
			{
				Weight: 2,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key: "apsara.metric.clusterName",
								Values: []string{
									dbClusterName,
								},
								Operator: metav1.LabelSelectorOpIn,
							},
							{
								Key: define.LabelInsType,
								Values: []string{
									configuration.GetConfig().ClusterInfoDbInsTypeRo,
								},
								Operator: metav1.LabelSelectorOpIn,
							},
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}
}

func GetNetworkCardName(logger logr.Logger, key string) string {
	netConfig := &corev1.ConfigMap{}
	netConfigMapNs := client.ObjectKey{
		Namespace: define.NetConfigMapNamespace,
		Name:      define.NetConfigmapName,
	}

	if err := manager.GetSyncClient().Get(context.TODO(), netConfigMapNs, netConfig); err != nil {
		logger.Error(err, fmt.Sprintf("system try to get config client network card err, so use default card %s!", define.DefaultClientNetCardName))
		return define.DefaultClientNetCardName
	}

	netCardName, ok := netConfig.Data[key]
	if !ok || netCardName == "" {
		return define.DefaultClientNetCardName
	}

	return netCardName
}
