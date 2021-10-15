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
	"strconv"
	"time"

	mgr "gitlab.alibaba-inc.com/polar-as/polar-common-domain/manager"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/business/domain"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/configuration"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/define"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/hardware_status"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/k8sutil"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/ssh"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var PodManagerNotInitError = errors.New("pod manager not init.")

type PodManagerBase struct {
	Ins    *domain.DbIns
	Logger logr.Logger
	Inited bool
}

func (q *PodManagerBase) SetIns(ins *domain.DbIns) {
	q.Ins = ins
}

func (q *PodManagerBase) DeletePod(ctx context.Context) error {
	if !q.Inited {
		return PodManagerNotInitError
	}
	err := k8sutil.DeletePod(q.Ins.ResourceName, q.Ins.ResourceNamespace, ctx, q.Logger)
	if err != nil {
		return err
	}
	q.Ins.Installed = false
	q.Ins.Host = ""
	q.Ins.HostIP = ""
	q.Ins.ClientIP = ""
	q.Ins.EngineState = nil

	return nil
}

/**
 * @Description: pod是否已经被删除
 * @param resourceName pod名称
 * @param ctx
 * @return error
 */
func (q *PodManagerBase) IsDeleted(ctx context.Context) (bool, error) {
	pod, err := k8sutil.GetPod(q.Ins.ResourceName, q.Ins.ResourceNamespace, q.Logger)
	if pod != nil && pod.DeletionTimestamp != nil {
		q.Logger.Info(fmt.Sprintf("find pod %s already deleted, DeletionTimestamp: %v ignore.", pod.Name, pod.DeletionTimestamp))
		return true, nil
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			q.Logger.Info(fmt.Sprintf("pod %s is not found, ignore", q.Ins.ResourceName))
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func (q *PodManagerBase) EnsureInsTypeMeta(ctx context.Context, insType string) error {
	pod, err := k8sutil.GetPod(q.Ins.ResourceName, q.Ins.ResourceNamespace, q.Logger)
	if err != nil {
		q.Logger.Error(err, "get pod failed.")
		return err
	}
	if pod == nil || pod.DeletionTimestamp != nil {
		errMsg := fmt.Sprintf("find pod %s already deleted, DeletionTimestamp: %v ignore.", pod.Name, pod.DeletionTimestamp)
		q.Logger.Info(errMsg)
		return errors.New(errMsg)
	}
	if t := pod.Labels[define.LabelInsType]; t != insType {
		pod.Labels[define.LabelInsType] = insType
		err := mgr.GetSyncClient().Update(context.TODO(), pod)
		if err != nil {
			q.Logger.Error(err, fmt.Sprintf("update pod [%s] label %s failed", pod.Name, define.LabelInsType))
			return err
		}
	}
	return nil
}

func (q *PodManagerBase) CleanDataFiles() error {
	logPath := configuration.GetConfig().DbClusterLogDir

	dataDir := fmt.Sprintf(`%s%s`, logPath, q.Ins.InsId)
	var cmd = fmt.Sprintf(`[ -d "%s/data" ] && cp -rp %s/data/ %s/rm_data_%s;rm -fr %s/data/*`, dataDir, dataDir, dataDir, time.Now().Format("20060102150405"), dataDir)
	nodes, err := k8sutil.GetNodes(q.Logger)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		execErr := ExecCommand(node.Name, q.Logger, func(out string, err error) bool {
			return err == nil
		}, cmd)
		if execErr != nil {
			q.Logger.Error(err, "clean up instance data failed", "node", node.Name)
		}
	}
	return nil
}

func ExecCommand(runCmdNode string, logger logr.Logger, checkSucceedFunc func(string, error) bool, cmdList ...string) error {
	for _, cmd := range cmdList {
		var err error
		sshOut, errMsg, sshErr := ssh.RunSSHNoPwdCMD(logger, cmd, runCmdNode)
		out := sshOut
		if sshErr != nil {
			err = sshErr
		} else if errMsg != "" {
			err = errors.New(errMsg)
		}

		if !checkSucceedFunc(out, err) {
			logger.Error(err, "execute cmd failed", "command", cmd, "put", out)
			return err
		}
	}
	return nil
}

func BuildAndRefVolumeInfo(pvcName, insId string, pod *v1.Pod) {
	hostPathType := corev1.HostPathDirectoryOrCreate
	logPath := configuration.GetConfig().DbClusterLogDir

	pod.Spec.Volumes = append(pod.Spec.Volumes, []corev1.Volume{
		{
			Name: "hugepage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumHugePages,
				},
			},
		}, {
			Name: "devshm",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		}, {
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: logPath + insId + "/data",
					Type: &hostPathType,
				},
			},
		}, {
			Name: "config-log",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: logPath + insId + "/log",
					Type: &hostPathType,
				},
			},
		}, {
			Name: "pfs-scripts",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{

					Path: logPath + insId + "/scripts",
					Type: &hostPathType,
				},
			},
		}, {
			Name: "devshmpfsd",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/dev/shm/pfsd",
					Type: &hostPathType,
				},
			},
		}, {
			Name: "runpfsd",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/pfsd",
					Type: &hostPathType,
				},
			},
		}, {
			Name: "var-run-pfs",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/pfs",
					Type: &hostPathType,
				},
			},
		}, {
			Name: "pfs-var-log",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/log/pfs_log",
					Type: &hostPathType,
				},
			},
		}, {
			Name: "common-pg-hba-cfg",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/postgres/",
					Type: &hostPathType,
				},
			},
		}, {
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		}, {
			Name: "polartrace-run",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/polartrace",
					Type: &hostPathType,
				},
			},
		}, {
			Name: "polartrace-shm",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/dev/shm/polartrace",
					Type: &hostPathType,
				},
			},
		},
	}...)
}

func BuildPfsdContainer(dbCluster *domain.SharedStorageDbClusterBase, conf *SysResourceConfig) corev1.Container {
	var (
		config               = configuration.GetConfig()
		mountPropagationMode = corev1.MountPropagationHostToContainer
		cpuLimit             = conf.ReadWriteMany.Pfsd.Limits.CPU
		memLimit             = conf.ReadWriteMany.Pfsd.Limits.Memory
		memReq               = memLimit
		cpuReq               = cpuLimit
	)

	c := corev1.Container{
		Name:            "pfsd",
		Image:           dbCluster.ImageInfo.Images[define.PfsdImageName],
		ImagePullPolicy: config.ImagePullPolicy,
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"SYS_PTRACE",
				},
			},
			Privileged: &config.RunPodInPrivilegedMode,
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memLimit,
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuReq,
				corev1.ResourceMemory: memReq,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config-data",
				MountPath: "/data",
				SubPath:   "",
			},
			{
				Name:      "config-log",
				MountPath: "/log",
				SubPath:   "",
			}, {
				Name:             "devshmpfsd",
				MountPath:        "/dev/shm/pfsd",
				SubPath:          "",
				MountPropagation: &mountPropagationMode,
			}, {
				Name:             "runpfsd",
				MountPath:        "/var/run/pfsd",
				SubPath:          "",
				MountPropagation: &mountPropagationMode,
			}, {
				Name:             "var-run-pfs",
				MountPath:        "/var/run/pfs",
				SubPath:          "",
				MountPropagation: &mountPropagationMode,
			}, {
				Name:      "pfs-var-log",
				MountPath: "/var/log",
				SubPath:   "",
			}, {
				Name:             "polartrace-run",
				MountPath:        "/var/run/polartrace",
				SubPath:          "",
				MountPropagation: &mountPropagationMode,
			}, {
				Name:             "polartrace-shm",
				MountPath:        "/dev/shm/polartrace",
				SubPath:          "",
				MountPropagation: &mountPropagationMode,
			}, {
				Name:      "pfs-scripts",
				MountPath: "/scripts",
				SubPath:   "",
			},
		},
		VolumeDevices: []corev1.VolumeDevice{
			{
				Name:       "data",
				DevicePath: "/dev/mapper/" + dbCluster.StorageInfo.VolumeId,
			},
		},
		Lifecycle: &v1.Lifecycle{
			PreStop: &v1.Handler{
				Exec: &v1.ExecAction{
					Command: []string{
						"/usr/local/polarstore/pfsd/bin/pre_stop_pfsd.sh",
						"mapper_" + dbCluster.StorageInfo.VolumeId,
					},
				},
			},
		},
	}
	return c
}

func BuildPfsdToolsContainer(dbCluster *domain.SharedStorageDbClusterBase, conf *SysResourceConfig) corev1.Container {
	var (
		config               = configuration.GetConfig()
		mountPropagationMode = corev1.MountPropagationHostToContainer
		cpuLimit             = conf.ReadWriteMany.PfsdTool.Limits.CPU
		memLimit             = conf.ReadWriteMany.PfsdTool.Limits.Memory
		memReq               = memLimit
		cpuReq               = cpuLimit
	)

	c := corev1.Container{
		Name:            "pfsd-tool",
		Image:           dbCluster.ImageInfo.Images[define.PfsdToolImageName],
		ImagePullPolicy: config.ImagePullPolicy,
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"SYS_PTRACE",
				},
			},
			Privileged: &config.RunPodInPrivilegedMode,
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memLimit,
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuReq,
				corev1.ResourceMemory: memReq,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config-data",
				MountPath: "/data",
				SubPath:   "",
			},
			{
				Name:      "config-log",
				MountPath: "/log",
				SubPath:   "",
			},
			{
				Name:             "devshm",
				MountPath:        "/dev/shm",
				SubPath:          "",
				MountPropagation: &mountPropagationMode,
			}, {
				Name:      "var-run-pfs",
				MountPath: "/var/run/pfs",
				SubPath:   "",
			}, {
				Name:      "pfs-var-log",
				MountPath: "/var/log",
				SubPath:   "",
			}, {
				Name:      "pfs-scripts",
				MountPath: "/scripts",
				SubPath:   "",
			},
		},
		VolumeDevices: []corev1.VolumeDevice{
			{
				Name:       "data",
				DevicePath: "/dev/mapper/" + dbCluster.StorageInfo.VolumeId,
			},
		},
	}
	return c
}

func BuildPodObjectMeta(ins domain.DbIns, cluster domain.SharedStorageDbClusterBase, insType string) metav1.ObjectMeta {
	podName := ins.ResourceName
	podNamespace := cluster.Namespace
	clusterName := cluster.Name
	objectMeta := metav1.ObjectMeta{
		Name:      podName,
		Namespace: podNamespace,
		Labels: map[string]string{
			define.LabelInsType:     insType,
			define.LabelClusterName: clusterName,
			define.LabelDbType: "polardb",
			define.LabelLogicCustInsName: clusterName,
			define.LabelInsName: ins.PhysicalInsId,
			define.LabelPvName: cluster.StorageInfo.VolumeId,
			define.LabelInsPort: strconv.Itoa(ins.NetInfo.Port),
			define.LabelLogicCustInsId: ins.InsId,
			define.LabelPhysicalCustInsId: ins.PhysicalInsId,
			define.LabelInsId: ins.InsId,
		},
		Annotations: map[string]string{},
	}
	return objectMeta
}

func BuildEngineContainer(logger logr.Logger, dbCluster *domain.SharedStorageDbClusterBase, ins *domain.DbIns, instanceClass *domain.EngineClass) corev1.Container {
	var (
		config           = configuration.GetConfig()
		mountPropagation = corev1.MountPropagationHostToContainer
		cpuLimit         *resource.Quantity
		memLimit         *resource.Quantity
		memReq           *resource.Quantity
		cpuReq           *resource.Quantity
	)
	cpuLimit = instanceClass.CpuLimit
	memLimit = instanceClass.MemoryLimit
	if cpuLimit != nil {
		cpuReq = cpuLimit
	}
	if memLimit != nil {
		memReq = memLimit
	}
	if instanceClass.CpuRequest != nil {
		cpuReq = instanceClass.CpuRequest
	}
	if instanceClass.MemoryRequest != nil {
		memReq = instanceClass.MemoryRequest
	}

	resourceLimit := corev1.ResourceList{
		//according to  http://gitlab.alibaba-inc.com/rds/fulong_rds_stack/blob/release_3.8.2.0/stacks/PUBLICRDS/2.9.1.1/services/DOCKER_ENGINE/package/scripts/engine/polardb_pg/1.0/engine_level.yaml
		corev1.ResourceHugePagesPrefix + "2Mi": *instanceClass.HugePageLimit,
	}
	if cpuLimit != nil {
		resourceLimit[corev1.ResourceCPU] = *cpuLimit
	}
	if memLimit != nil {
		resourceLimit[corev1.ResourceMemory] = *memLimit
	}

	logPath := configuration.GetConfig().DbClusterLogDir

	c := corev1.Container{
		Name:            "engine",
		Image:           dbCluster.ImageInfo.Images[define.EngineImageName],
		ImagePullPolicy: configuration.GetConfig().ImagePullPolicy,
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"SYS_PTRACE",
				},
			},
			Privileged: &config.RunPodInPrivilegedMode,
		},
		Resources: corev1.ResourceRequirements{
			Limits: resourceLimit,
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    *cpuReq,
				corev1.ResourceMemory: *memReq,
			},
		},
		VolumeDevices: []corev1.VolumeDevice{
			{
				Name:       "data",
				DevicePath: "/dev/mapper/" + dbCluster.StorageInfo.VolumeId,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "hugepage",
				MountPath: "/hugepages",
			},
			{
				Name:      "config-data",
				MountPath: "/data",
			},
			{
				Name:      "config-log",
				MountPath: "/log",
			}, {
				Name:             "devshm",
				MountPath:        "/dev/shm",
				SubPath:          "",
				MountPropagation: &mountPropagation,
			},
			{
				Name:             "common-pg-hba-cfg",
				MountPath:        "/etc/postgres/",
				SubPath:          "",
				MountPropagation: &mountPropagation,
				ReadOnly:         true,
			},
			{
				Name:             "devshmpfsd",
				MountPath:        "/dev/shm/pfsd",
				SubPath:          "",
				MountPropagation: &mountPropagation,
			}, {
				Name:             "runpfsd",
				MountPath:        "/var/run/pfsd",
				SubPath:          "",
				MountPropagation: &mountPropagation,
			}, {
				Name:      "var-run-pfs",
				MountPath: "/var/run/pfs",
				SubPath:   "",
			}, {
				Name:      "pfs-var-log",
				MountPath: "/var/log",
				SubPath:   "",
			}, {
				Name:      "pfs-scripts",
				MountPath: "/scripts",
				SubPath:   "",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: int32(ins.NetInfo.Port),
			},
		},
		Lifecycle: &v1.Lifecycle{
			PreStop: &v1.Handler{
				Exec: &v1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-c",
						"srv_opr_type=hostins_ops srv_opr_action=process_cleanup /docker_script/entry_point.py",
					},
				},
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "host_data_dir",
				Value: logPath + ins.InsId + "/data",
			},
			{
				Name:  "host_log_dir",
				Value: logPath + ins.InsId + "/log",
			},
		},
	}
	return c
}

func BuildManagerContainer(dbCluster *domain.SharedStorageDbClusterBase, conf *SysResourceConfig) corev1.Container {
	var (
		mountPropagation = corev1.MountPropagationHostToContainer
		config           = configuration.GetConfig()
		cpuLimit         = conf.ReadWriteMany.Manager.Limits.CPU
		memLimit         = conf.ReadWriteMany.Manager.Limits.Memory
		cpuReq           = conf.ReadWriteMany.Manager.Requests.CPU
		memReq           = conf.ReadWriteMany.Manager.Requests.Memory
	)

	c := corev1.Container{
		Name:            "manager",
		Image:           dbCluster.ImageInfo.Images[define.ManagerImageName],
		ImagePullPolicy: config.ImagePullPolicy,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memLimit,
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuReq,
				corev1.ResourceMemory: memReq,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"SYS_PTRACE",
				},
			},
			Privileged: &config.RunPodInPrivilegedMode,
		},
		VolumeDevices: []corev1.VolumeDevice{
			{
				Name:       "data",
				DevicePath: "/dev/mapper/" + dbCluster.StorageInfo.VolumeId,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config-data",
				MountPath: "/data",
			},
			{
				Name:      "config-log",
				MountPath: "/log",
			},
			{
				Name:             "devshmpfsd",
				MountPath:        "/dev/shm/pfsd",
				SubPath:          "",
				MountPropagation: &mountPropagation,
			}, {
				Name:             "runpfsd",
				MountPath:        "/var/run/pfsd",
				SubPath:          "",
				MountPropagation: &mountPropagation,
			}, {
				Name:      "var-run-pfs",
				MountPath: "/var/run/pfs",
				SubPath:   "",
			}, {
				Name:      "pfs-var-log",
				MountPath: "/var/log",
				SubPath:   "",
			},
			{
				Name:             "common-pg-hba-cfg",
				MountPath:        "/etc/postgres/",
				SubPath:          "",
				MountPropagation: &mountPropagation,
				ReadOnly:         true,
			}, {
				Name:      "pfs-scripts",
				MountPath: "/scripts",
				SubPath:   "",
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "logic_ins_id",
				Value: dbCluster.LogicInsId,
			},
		},
	}
	return c
}

func BuildPodNodeAffinity(taintNodeList []string, targetNode string, logger logr.Logger) (*corev1.NodeAffinity, error) {
	controllerConf, err := k8sutil.GetControllerConfig(logger)
	if err != nil {
		err = errors.Errorf("GetControllerConfig error:[%v]", err)
		logger.Error(err, "")
		return nil, err
	}

	// 非混部，部署到非管控节点
	var hybridSelectRequirement *corev1.NodeSelectorRequirement = nil
	if !controllerConf.HybridDeployment && controllerConf.ControllerNodeLabel != "" {
		hybridSelectRequirement = &corev1.NodeSelectorRequirement{
			Key:      controllerConf.ControllerNodeLabel,
			Operator: corev1.NodeSelectorOpDoesNotExist,
		}
	}

	// 故障节点
	var taintNodeRequirement *corev1.NodeSelectorRequirement = nil
	if taintNodeList != nil && len(taintNodeList) > 0 {
		taintNodeRequirement = &corev1.NodeSelectorRequirement{
			Key:      "kubernetes.io/hostname",
			Operator: corev1.NodeSelectorOpNotIn,
			Values:   taintNodeList,
		}
	}

	var targetNodeSelectRequirement *corev1.NodeSelectorRequirement = nil
	if targetNode != "" {
		//有设置目标node并且目标node不为空
		if utils.ContainsString(taintNodeList, targetNode, nil) {
			//目标Node在故障节点内
			errMsg := fmt.Sprintf("target node [%s] is unavailable!", targetNode)
			err := define.CreateInterruptError(define.TargetNodeUnavailable, errors.New(errMsg))
			logger.Error(err, "")
			//出现互诉情况，直接中断
			return nil, err
		} else {
			targetNodeSelectRequirement = &corev1.NodeSelectorRequirement{
				Key:      "kubernetes.io/hostname",
				Operator: corev1.NodeSelectorOpIn,
				Values: []string{
					targetNode,
				},
			}
		}
	}

	var matchExpress []corev1.NodeSelectorRequirement
	if targetNodeSelectRequirement != nil {
		matchExpress = append(matchExpress, *targetNodeSelectRequirement)
	}
	if hybridSelectRequirement != nil {
		matchExpress = append(matchExpress, *hybridSelectRequirement)
	}
	if taintNodeRequirement != nil {
		matchExpress = append(matchExpress, *taintNodeRequirement)
	}
	// 运维节点
	matchExpress = append(matchExpress, corev1.NodeSelectorRequirement{
		Key:      configuration.GetConfig().NodeMaintainLabelName,
		Operator: corev1.NodeSelectorOpDoesNotExist,
	})

	//不可用节点设置亲和性
	nodeAffinity := &corev1.NodeAffinity{}
	nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
		NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: matchExpress,
			},
		},
	}

	logger.Info("get pod nodeAffinity", "nodeAffinity", nodeAffinity)

	return nodeAffinity, nil
}

func BuildNodeAvailableInfo(logger logr.Logger) ([]string, error) {
	unAvailableNodes, availableNodes, err := GetUnAvailableNodes(logger)
	if err != nil {
		logger.Error(err, "check unAvailableNode error")
		return nil, err
	}

	if len(availableNodes) < 1 {
		err := errors.New("check unAvailableNode err: availableNodes num<1")
		logger.Error(err, "")
		return nil, define.CreateInterruptError(define.NoAvailableNode, err)
	}

	var taintNodeList []string

	logger.Info("check unAvailableNode", "availableNodes", GetNodeName(availableNodes), "unAvailableNode", GetNodeName(unAvailableNodes))
	if unAvailableNodes == nil || len(unAvailableNodes) < 1 {
		//无不可用节点
	} else {
		for _, node := range unAvailableNodes {
			taintNodeList = append(taintNodeList, node.Name)
		}
	}
	return taintNodeList, nil
}

func GetNodeName(nodes []corev1.Node) []string {
	var names []string
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}

func GetUnAvailableNode(logger logr.Logger) ([]string, error) {
	var taintNodeList []string
	unAvailableNodes, _, err := GetUnAvailableNodes(logger)
	if err != nil {
		logger.Error(err, "check unAvailableNode error")
		return nil, err
	}
	if unAvailableNodes != nil {
		for _, node := range unAvailableNodes {
			taintNodeList = append(taintNodeList, node.Name)
		}
	}
	return taintNodeList, nil
}

func GetUnAvailableNodes(logger logr.Logger) ([]corev1.Node, []corev1.Node, error) {
	nodes, err := k8sutil.GetNodes(logger)
	if err != nil {
		return nil, nil, err
	}
	if len(nodes) < 1 {
		return nil, nil, nil
	}

	var unAvailableNode []corev1.Node
	var availableNode []corev1.Node

	if configuration.GetConfig().HwCheck {
		unAvailableNode, availableNode = CheckHwStatusBySshNode(nodes, logger)
	} else {
		unAvailableNode, availableNode = CheckHwStatusByNodeCondition(nodes, logger)
	}

	return unAvailableNode, availableNode, nil
}

func CheckHwStatusBySshNode(nodes []corev1.Node, logger logr.Logger) (unAvailableNode []corev1.Node, availableNode []corev1.Node) {
	lenNodes := len(nodes)
	endChan := make(chan int, 1)
	defer close(endChan)

	resultChan := make(chan NodeAvailable, lenNodes)
	defer close(resultChan)

	///只选不可用的
	for i := 0; i < lenNodes; i++ {
		go GetNodeStatus(resultChan, &(nodes[i]), logger)
	}

	go func() {
		for i := 0; i < lenNodes; i++ {
			availableNodeInfo := <-resultChan
			if availableNodeInfo.Available {
				availableNode = append(availableNode, *availableNodeInfo.Node)
			} else {
				unAvailableNode = append(unAvailableNode, *availableNodeInfo.Node)
			}

		}
		endChan <- 1
	}()

	<-endChan

	return
}

func GetNodeStatus(resultChan chan NodeAvailable, node *corev1.Node, logger logr.Logger) {
	logger.Info(fmt.Sprintf("begin GetNodeStatus [%s]", node.Name))

	hostStatus := hardware_status.GetHostStatus(logger, node)

	logger.Info(fmt.Sprintf("GetNodeStatus [%s] is %+v.", node.Name, hostStatus))

	if hostStatus.HostStatus == hardware_status.InterfaceStatusOnLine {
		resultChan <- NodeAvailable{
			Node:      node,
			Available: true,
		}
	} else {
		resultChan <- NodeAvailable{
			Node:      node,
			Available: false,
		}
	}

}

func CheckHwStatusByNodeCondition(nodes []corev1.Node, logger logr.Logger) (unAvailableNode []corev1.Node, availableNode []corev1.Node) {
	// check node conditions
	for _, node := range nodes {
		storageCondition := k8sutil.GetNodeCondition(&node, define.StorageConditionType)
		clientNetCondition := k8sutil.GetNodeCondition(&node, define.ClientNetworkConditionType)
		if !CheckConditionAvailable(node.Name, storageCondition, logger) || !CheckConditionAvailable(node.Name, clientNetCondition, logger) {
			unAvailableNode = append(unAvailableNode, node)
			continue
		}
		availableNode = append(availableNode, node)
		logger.Info(fmt.Sprintf("%s node is available. storage: [%+v], clientNet: [%+v]", node.Name, k8sutil.StringOfCondition(storageCondition), k8sutil.StringOfCondition(clientNetCondition)))
	}
	return
}

func CheckConditionAvailable(nodeName string, cond *corev1.NodeCondition, logger logr.Logger) bool {
	if cond != nil {
		if cond.Status == corev1.ConditionTrue {
			logger.Info(fmt.Sprintf("%s node is unAvailable. because of [%s] is true: cond: [%+v] ", nodeName, cond.Type, k8sutil.StringOfCondition(cond)))
			return false
		}
	}
	return true
}

type NodeAvailable struct {
	Node      *corev1.Node
	Available bool
}
