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


package test

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	r "runtime"
	"strings"
	"testing"

	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/configuration"

	. "github.com/bouk/monkey"
	"github.com/go-logr/logr"
	. "github.com/golang/mock/gomock"
	mgr "gitlab.alibaba-inc.com/polar-as/polar-common-domain/manager"
	mock "gitlab.alibaba-inc.com/polar-as/polar-common-domain/test/mock"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/k8sutil"
	"gitlab.alibaba-inc.com/polar-as/polar-common-domain/utils/ssh"
	wfk8sutil "gitlab.alibaba-inc.com/polar-as/polar-wf-engine/utils/k8sutil"
	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8sFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/yaml"
)

var configmaps []*apicorev1.ConfigMap
var nodes []*apicorev1.Node
var services []*apicorev1.Service
var secrets []*apicorev1.Secret
var pods []*apicorev1.Pod

// FakeContainer ...
type FakeContainer struct {
	FakeClient                    client.Client
	FakeClientset                 *k8sFake.Clientset
	MockManager                   *mock.MockManager
	MockClassQuery                *mock.MockIClassQuery
	MockEngineParamsClassQuery    *mock.MockIEngineParamsClassQuery
	MockEngineParamsTemplateQuery *mock.MockIEngineParamsTemplateQuery
	MockMinorVersionQuery         *mock.MockIMinorVersionQuery
	Ctrl                          *Controller
	MockSmsServer                 *httptest.Server
	MockCmServer                  *httptest.Server
	CoreV1Client                  corev1.CoreV1Interface
	ExecInPodGuard                *PatchGuard
}

func init() {
	// chdir project dir to base
	_, filename, _, _ := r.Caller(0)
	rootDir := path.Join(path.Dir(filename), "..")
	err := os.Chdir(rootDir)
	if err != nil {
		panic(err)
	}

	testing.Init()
	flag.CommandLine.String("log_dir", "", "")

	configmaps = loadConfigMapResources()
	nodes = loadNodeResources()
	services = loadServiceResources()
	secrets = loadSecretResources()
	pods = loadPodResources()
	_ = scheme.AddToScheme(scheme.Scheme)

	configuration.SetConfig(&configuration.Config{
		DbClusterLogDir:              "/disk1/polardb/",
		CmCpuReqLimit:                "200m/500m",
		CmMemReqLimit:                "200Mi/512Mi",
		HwCheck:                      true,
		ImagePullPolicy:              "Always",
		RunPodInPrivilegedMode:       false,
		ClusterInfoDbInsTypeRw:       "polardb_rw",
		ClusterInfoDbInsTypeRo:       "polardb_ro",
		ClusterInfoDbInsTypeStandby:  "polardb_standby",
		ClusterInfoDbInsTypeDataMax:  "polardb_datamax",
		ParamsTemplateName:           "polar-o-1-0-mycnf-template",
		MinorVersionCmName:           "polar-o-1-0-minor-version-info",
		ExternalPortRangeAnnotation:  "external.port.range",
		ReservedPortRangesAnnotation: "reserved.port.ranges",
		PortRangeCmName:              "polardb-controller",
		InsIdRangeCmName:             "polardb-controller",
		NodeMaintainLabelName:        "polardb.aliyun.com/maintainNode",
		AccountMetaClusterLabelName:  "cluster_name",
		DbTypeLabel:                  "polardb_o",
		ControllerManagerRoleName:    "polardb-controller-manager-role",
	})
}

// InitFakeContainer ...
func InitFakeContainer(t *testing.T) (container *FakeContainer) {
	container = &FakeContainer{
		FakeClient:    clientFake.NewFakeClientWithScheme(scheme.Scheme),
		FakeClientset: k8sFake.NewSimpleClientset(),
	}
	container.CoreV1Client = container.FakeClientset.CoreV1()

	for _, obj := range nodes {
		err := container.FakeClient.Create(context.TODO(), obj)
		if err != nil {
			fmt.Println(err)
		}
		_, err = container.FakeClientset.CoreV1().Nodes().Create(obj)
		if err != nil {
			fmt.Println(err)
		}
	}

	for _, obj := range pods {
		err := container.FakeClient.Create(context.TODO(), obj)
		if err != nil {
			fmt.Println(err)
		}
		_, err = container.FakeClientset.CoreV1().Pods(obj.Namespace).Create(obj)
		if err != nil {
			fmt.Println(err)
		}
	}

	for _, obj := range configmaps {
		err := container.FakeClient.Create(context.TODO(), obj)
		if err != nil {
			fmt.Println(err)
		}
	}

	for _, obj := range services {
		err := container.FakeClient.Create(context.TODO(), obj)
		if err != nil {
			fmt.Println(err)
		}
	}

	for _, obj := range secrets {
		err := container.FakeClient.Create(context.TODO(), obj)
		if err != nil {
			fmt.Println(err)
		}
	}

	ctrl := NewController(t)
	defer ctrl.Finish()

	container.MockManager = mock.NewMockManager(ctrl)
	container.MockClassQuery = mock.NewMockIClassQuery(ctrl)
	container.MockEngineParamsClassQuery = mock.NewMockIEngineParamsClassQuery(ctrl)
	container.MockEngineParamsTemplateQuery = mock.NewMockIEngineParamsTemplateQuery(ctrl)
	container.MockMinorVersionQuery = mock.NewMockIMinorVersionQuery(ctrl)
	container.Ctrl = ctrl

	return
}

// PatchClient ...
func PatchClient(t *testing.T, action func(container *FakeContainer)) {
	container := InitFakeContainer(t)

	guardGetManager := Patch(mgr.GetManager, func() manager.Manager {
		return container.MockManager
	})
	defer guardGetManager.Unpatch()

	guardSSH := Patch(ssh.RunSSHNoPwdCMD, func(logger logr.Logger, cmd string, ipaddr string) (string, string, error) {
		return "", "", nil
	})
	defer guardSSH.Unpatch()

	guardSetControllerReference := Patch(controllerutil.SetControllerReference, func(owner, object metav1.Object, scheme *runtime.Scheme) error {
		return nil
	})
	defer guardSetControllerReference.Unpatch()

	guard := Patch(mgr.GetSyncClient, func() client.Client {
		return container.FakeClient
	})
	defer guard.Unpatch()

	guardCoreV1 := PatchInstanceMethod(reflect.TypeOf(&kubernetes.Clientset{}), "CoreV1", func(_ *kubernetes.Clientset) corev1.CoreV1Interface {
		return container.CoreV1Client
	})
	defer guardCoreV1.Unpatch()

	guardGetCoreV1 := Patch(k8sutil.GetCorev1Client, func() (corev1.CoreV1Interface, error) {
		return container.CoreV1Client, nil
	})
	defer guardGetCoreV1.Unpatch()

	guardExecInPod := Patch(k8sutil.ExecInPod, func(command []string, containerName, podName, namespace string, logger logr.Logger) (string, string, error) {
		fmt.Printf("%s [ %s - %s ]", command, podName, containerName)
		return "", "", nil
	})
	container.ExecInPodGuard = guardExecInPod
	defer guardExecInPod.Unpatch()

	guardCheckPodReady := Patch(k8sutil.CheckPodReady, func(name, namespace string, logger logr.Logger) bool {
		return true
	})
	defer guardCheckPodReady.Unpatch()

	guardWfGetCorev1Client := Patch(wfk8sutil.GetCorev1Client, func() (corev1.CoreV1Interface, error) {
		var objs []runtime.Object
		cs := k8sFake.NewSimpleClientset(objs...)
		return cs.CoreV1(), nil
	})
	defer guardWfGetCorev1Client.Unpatch()

	container.MockManager.EXPECT().GetScheme().AnyTimes().Return(nil)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/pvcs/use" {
			w.Write([]byte("{\"workflow_id\": \"workflow-id\"}"))
		} else if r.URL.Path == "/pvcs/ready" {
			w.Write([]byte("{\"is_ready\": true}"))
		} else if r.URL.Path == "/pvcs/lock" {
			w.Write([]byte("{\"name\": \"36e00084100ee7ec9727c50ec00001f4c\",\"write_lock_node_id\": \"r03.dbm-02\", \"write_lock_node_ip\": \"\"}"))
		} else if r.URL.Path == "/pvcs/topo" {
			w.Write([]byte("{\"read_nodes\": [{\"node_id\": \"r03.dbm-01\", \"node_ip\": \"198.19.64.1\"}], \"write_node\": {\"node_id\": \"r03.dbm-02\", \"node_ip\": \"198.19.64.2\"}}"))
		} else if r.URL.Path == "/pvcs/expand" && r.Method == "POST" {
			defer r.Body.Close()
			body, _ := ioutil.ReadAll(r.Body)
			fmt.Print(string(body))
			if strings.Contains(string(body), "failed") {
				w.Write([]byte("{\"workflow_id\": \"workflow-id-failed\"}"))
				return
			} else if strings.Contains(string(body), "processing") {
				w.Write([]byte("{\"workflow_id\": \"workflow-id-processing\"}"))
				return
			}
			w.Write([]byte("{\"workflow_id\": \"workflow-id-succeed\"}"))
		} else if r.URL.Path == "/pvcs/expand" && r.Method == "GET" {
			if strings.Contains(r.URL.RawQuery, "failed") {
				w.Write([]byte("{\"status\": 2, \"err_message\": \"expand error!!\"}"))
				return
			} else if strings.Contains(r.URL.RawQuery, "processing") {
				w.Write([]byte("{\"status\": 1}"))
				return
			}
			w.Write([]byte("{\"status\": 0}"))
		}
	}))
	defer ts.Close()
	container.MockSmsServer = ts

	cmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/global_add_datamax" {
			w.Write([]byte("{\"code\": 200}"))
		} else if r.URL.Path == "/v1/global_update_datamax" {
			w.Write([]byte("{\"code\": 200}"))
		} else if r.URL.Path == "/v1/global_remove_datamax" {
			w.Write([]byte("{\"code\": 200}"))
		} else if r.URL.Path == "/v1/global_add_cluster" {
			w.Write([]byte("{\"code\": 200}"))
		} else if r.URL.Path == "/v1/global_update_cluster" {
			w.Write([]byte("{\"code\": 200}"))
		} else if r.URL.Path == "/v1/global_remove_cluster" {
			w.Write([]byte("{\"code\": 200}"))
		} else if r.URL.Path == "/v1/global_add_topology_edge" {
			w.Write([]byte("{\"code\": 200}"))
		} else if r.URL.Path == "/v1/global_remove_topology_edge" {
			w.Write([]byte("{\"code\": 200}"))
		}
	}))
	defer cmServer.Close()
	container.MockCmServer = cmServer

	action(container)
}

func loadNodeResources() (objs []*apicorev1.Node) {
	items := loadTypedResources("node", func() interface{} { return &apicorev1.Node{} })
	for _, item := range items {
		objs = append(objs, item.(*apicorev1.Node))
	}
	return
}

func loadPodResources() (objs []*apicorev1.Pod) {
	items := loadTypedResources("pod", func() interface{} { return &apicorev1.Pod{} })
	for _, item := range items {
		objs = append(objs, item.(*apicorev1.Pod))
	}
	return
}

func loadSecretResources() (objs []*apicorev1.Secret) {
	items := loadTypedResources("secret", func() interface{} { return &apicorev1.Secret{} })
	for _, item := range items {
		objs = append(objs, item.(*apicorev1.Secret))
	}
	return
}

func loadServiceResources() (objs []*apicorev1.Service) {
	items := loadTypedResources("service", func() interface{} { return &apicorev1.Service{} })
	for _, item := range items {
		objs = append(objs, item.(*apicorev1.Service))
	}
	return
}

func loadConfigMapResources() (objs []*apicorev1.ConfigMap) {
	items := loadTypedResources("configmap", func() interface{} { return &apicorev1.ConfigMap{} })
	for _, item := range items {
		objs = append(objs, item.(*apicorev1.ConfigMap))
	}
	return
}

func loadTypedResources(subDir string, createObject func() interface{}) (objs []interface{}) {
	dir := "test/data/k8s"
	if subDir != "" {
		dir = dir + "/" + subDir
	}
	result := loadFromDir(dir)
	for _, bytes := range result {
		obj := createObject()
		err := yaml.Unmarshal(bytes, obj)
		if err != nil {
			fmt.Println(err)
			return
		}
		objs = append(objs, obj)
	}
	return
}

func loadFromDir(dir string) (objs [][]byte) {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			dirInfo, _ := ioutil.ReadDir(path)
			for _, fileInfo := range dirInfo {
				objs = append(objs, loadFromDir(path+"/"+fileInfo.Name())...)
			}
		}
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Println(err)
			return err
		}

		objs = append(objs, bytes)

		return nil
	})
	return
}
