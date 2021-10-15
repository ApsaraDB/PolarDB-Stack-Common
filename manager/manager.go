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


package manager

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	mgr manager.Manager
)

func RegisterManager(m manager.Manager) {
	mgr = m
}

func GetManager() manager.Manager {
	return mgr
}

func GetSyncClient() client.Client {
	clt := mgr.GetClient()
	cc := clt.(*client.DelegatingClient).Reader.(*client.DelegatingReader).ClientReader
	return cc.(client.Client)
}
