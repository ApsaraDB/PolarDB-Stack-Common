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


package k8sutil

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
)

var ClientForTest corev1.CoreV1Interface = nil 

// GetKubeClient ...
func GetKubeClient() (*kubernetes.Clientset, *rest.Config, error) {
	config, err := GetKubeConfig()
	if err != nil {
		return nil, config, errors.Wrap(err, "GetKubeClient error")
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, config, errors.Wrap(err, "GetKubeClient")
	}
	return clientSet, config, nil
}

func GetCorev1Client() (corev1.CoreV1Interface, error) {
	if ClientForTest != nil {
		return ClientForTest, nil
	}
	_, config, err := GetKubeClient()
	if err != nil {
		return nil, errors.Wrap(err, "GetCorev1Client error")
	}

	coreV1Client, err := corev1.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "corev1client.NewForConfig error")
	}

	return coreV1Client, nil
}

// GetDynamicClient ...
func GetDynamicClient(group, version, resource string) (dynamic.NamespaceableResourceInterface, error) {
	cfg, err := GetKubeConfig()
	if err != nil {
		return nil, err
	}
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create dynamic client")
	}
	return dc.Resource(schema.GroupVersionResource{Group: group, Version: version, Resource: resource}), nil
}

// GetKubeConfig ...
func GetKubeConfig() (*rest.Config, error) {
	var cfg *rest.Config
	manager := mgr.GetManager()
	if manager != nil {
		cfg = manager.GetConfig()
	} else {
		conf, err := config.GetConfig()
		if err != nil {
			klog.Error(err, "unable to set up client config")
			return nil, fmt.Errorf("could not get kubernetes config from kubeconfig: %v", err)
		}
		cfg = conf
	}
	return cfg, nil
}
