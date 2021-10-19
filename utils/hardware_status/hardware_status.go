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


package hardware_status

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/go-logr/logr"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/ssh"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ApsaraDB/PolarDB-Stack-Common/manager"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"
)

type InterfaceStatus string

type HostStatus struct {
	HostName              string
	ClientInterfaceName   string
	ClientInterfaceStatus InterfaceStatus
	HostStatus            InterfaceStatus
	ErrMsg                string
	testTime              time.Time
}

const (
	InterfaceStatusOnLine  = "OnLine"
	InterfaceStatusOffLine = "OffLine"
	InterfaceStatusUnknown = "Unknown"
)

type hostTestingMapping map[string]*sync.Mutex

var hostTestingLockerMap = make(hostTestingMapping)
var hostTestingLockerMapLocker = sync.Mutex{}
var hostStatusMap = make(map[string]*HostStatus)
var hostStatusLockerMap = make(hostTestingMapping)
var hostStatusLockerMapLocker = sync.Mutex{}

func GetHostStatus(logger logr.Logger, node *corev1.Node) *HostStatus {
	if node == nil {
		return nil
	}
	l := getHostStatusLocker(node.Name)
	l.Lock()
	defer l.Unlock()

	hostStatus, ok := hostStatusMap[node.Name]

	if ok {
		//状态3秒内有效
		if time.Now().Sub(hostStatus.testTime).Seconds() <= 3 {
			logger.Info(fmt.Sprintf("GetHostStatus [%s], using cached value: %+v ", node.Name, hostStatus))
			return hostStatus
		} else {
			logger.Info(fmt.Sprintf("GetHostStatus [%s], timeOuted value: %+v, probe it.", node.Name, hostStatus))
			delete(hostStatusMap, node.Name)
			//状态已超时，需要重新采集
		}
	}

	begin := time.Now()
	defer func() {
		logger.Info("GetHostStatus [%s], end probe status! spend=%v s", node.Name, time.Now().Sub(begin).Seconds())
	}()

	logger.Info(fmt.Sprintf("GetHostStatus [%s], begin probe status.", node.Name))
	newStatus := testNodeStatus(node, logger)
	newStatus.testTime = time.Now()

	hostStatusMap[node.Name] = newStatus

	logger.Info("GetHostStatus [%s], probe status result is: %+v", node.Name, newStatus)
	return newStatus
}

func testNodeStatus(node *corev1.Node, logger logr.Logger) *HostStatus {
	hostName := node.Name
	locker := getTestHostTestLocker(hostName)

	locker.Lock()
	defer locker.Unlock()
	return testHostHardwareStatus(node, logger)
}

func getTestHostTestLocker(hostName string) *sync.Mutex {
	hostTestingLockerMapLocker.Lock()
	defer hostTestingLockerMapLocker.Unlock()

	testLocker, ok := hostTestingLockerMap[hostName]

	if ok {
		return testLocker
	}

	locker := &sync.Mutex{}
	hostTestingLockerMap[hostName] = locker
	return locker
}

func getHostStatusLocker(hostName string) *sync.Mutex {
	hostStatusLockerMapLocker.Lock()
	defer hostStatusLockerMapLocker.Unlock()

	l, ok := hostStatusLockerMap[hostName]

	if ok {
		return l
	}

	locker := &sync.Mutex{}
	hostStatusLockerMap[hostName] = locker
	return locker
}

func testHostHardwareStatus(node *corev1.Node, logger logr.Logger) *HostStatus {
	clientCardName := getClientNetworkCardName(logger)
	logger.Info(fmt.Sprintf("system get the client network card name is %s", clientCardName))

	status := &HostStatus{
		HostName:              node.Name,
		ClientInterfaceName:   clientCardName,
		ClientInterfaceStatus: InterfaceStatusUnknown,
	}

	if !k8sutil.IsNodeReady(node) {
		isSshLived := utils.CheckAddressSSHLived(node.Name, 3*time.Second, logger)
		if !isSshLived {
			//节点已不存活
			logger.Info(fmt.Sprintf("check the node %s is dead, add to unAvailableNode", node.Name))
			status.HostStatus = InterfaceStatusOffLine
			status.ErrMsg = "Node is NotReady and ssh not lived!"
			return status
		}
	}

	nodeStatus := false

	endChan := make(chan int, 1)
	defer close(endChan)

	go func() {
		defer func() {
			endChan <- 1
		}()
		nodeStatus = checkNodeStatusAvailable(node, clientCardName, logger)
	}()

	<-endChan

	if nodeStatus {
		status.ClientInterfaceStatus = InterfaceStatusOnLine
	} else {
		status.ClientInterfaceStatus = InterfaceStatusOffLine
	}

	if nodeStatus {
		status.HostStatus = InterfaceStatusOnLine
	} else {
		status.HostStatus = InterfaceStatusOffLine
		status.ErrMsg = "Host client interface and san conn has some err"
	}

	return status
}

func getClientNetworkCardName(logger logr.Logger) string {
	netConfig := &corev1.ConfigMap{}
	netConfigMapNs := client.ObjectKey{
		Namespace: "kube-system",
		Name:      "ccm-config",
	}
	defaultBondName := "bond1"
	if err := manager.GetSyncClient().Get(context.TODO(), netConfigMapNs, netConfig); err != nil {
		logger.Error(err, fmt.Sprintf("system try to get config client network card err, so use default card %s.", defaultBondName))
		return defaultBondName
	}
	netcardName, ok := netConfig.Data["NET_CARD_NAME"]
	if !ok {
		return defaultBondName
	}
	if netcardName == "" {
		return defaultBondName
	}
	return netcardName
}

func checkNodeStatusAvailable(node *corev1.Node, clientNetworkCardName string, logger logr.Logger) bool {
	addressList := node.Status.Addresses
	var nodeAddress = ""
	for _, adr := range addressList {
		if adr.Type == corev1.NodeHostName {
			nodeAddress = adr.Address
			break
		}
	}

	if nodeAddress == "" {
		err := errors.New(fmt.Sprintf("node [%s] node Address is nil [%s], use name instead.", node.Name, nodeAddress))
		logger.Error(err, "")
		nodeAddress = node.Name
	}

	isNodeReady := k8sutil.IsNodeReady(node)
	if !isNodeReady {
		err := errors.New(fmt.Sprintf("node [%s] condition.Status.NodeReady is %v.", node.Name, isNodeReady))
		logger.Error(err, "")
		return false
	}

	checkNicStatusCmd := fmt.Sprintf("ip a show %s|grep \" state UP \"|grep %s: ", clientNetworkCardName, clientNetworkCardName)
	stdOut, errInfo, err := ssh.RunSSHNoPwdCMD(logger, checkNicStatusCmd, nodeAddress)

	if err != nil || errInfo != "" && !strings.Contains(errInfo, "Warn") {
		logger.Error(err, fmt.Sprintf("node [%s] card [%s]  check is error and still check output", node.Name, clientNetworkCardName))
	}

	if strings.Contains(stdOut, "state UP") {
		return true
	}

	if strings.Contains(stdOut, "state DOWN") {
		return false
	}

	logger.Error(errors.New(fmt.Sprintf("node [%s] card [%s]  check is unknown", node.Name, clientNetworkCardName)), "")

	return false
}
