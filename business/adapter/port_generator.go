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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/ApsaraDB/PolarDB-Stack-Common/configuration"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ApsaraDB/PolarDB-Stack-Common/define"
	"github.com/ApsaraDB/PolarDB-Stack-Common/manager"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"
	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/ssh"
)

func NewPortGenerator(logger logr.Logger) *PortGenerator {
	return &PortGenerator{
		logger: logger.WithValues("component", "PortGenerator"),
	}
}

type PortGenerator struct {
	logger logr.Logger
	mutex  sync.Mutex
}

func (g *PortGenerator) GetNextClusterExternalPort() (int, error) {
	for re := 0; re < 3; re++ {
		port, err := g.getNextScopePort(define.NextExternalPortKey, configuration.GetConfig().ExternalPortRangeAnnotation)
		if err != nil && err.Error() == "CAN_NOT_UPDATE" {
			g.logger.Info("retry GetNextClusterExternalPort", "times", re+1)
			continue
		}
		return port, err
	}
	return 0, errors.New("can not update NextClusterScopePort")
}

func (g *PortGenerator) CheckPortUsed(port int, rangesName string) (bool, error) {
	// 验证端口是否在规定范围内
	foundConfigMap, err := getPortConfigMap(g.logger)
	if err != nil {
		return true, err
	}
	reservedPortRange := ""
	if rpr, ok := foundConfigMap.Annotations[configuration.GetConfig().ReservedPortRangesAnnotation]; ok {
		reservedPortRange = rpr
	}
	prs, err := g.parsePortRanges(foundConfigMap.Annotations[rangesName], reservedPortRange)
	if err != nil {
		return true, err
	}
	if !prs.Contains(port) {
		return true, errors.New(fmt.Sprintf("port %d is not in the range [%s]", port, rangesName))
	}
	// 验证是否被占用
	usedPort, err := g.getAllNodeUsedPort()
	if err != nil {
		g.logger.Error(err, "get the port used info error")
		return true, err
	}
	if usedPort.Contains(int32(port)) {
		return true, errors.New(fmt.Sprintf("port %d is already occupied", port))
	}
	return false, nil
}

func (g *PortGenerator) getNextScopePort(cmKey string, beginPortKey string) (int, error) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	foundConfigMap, err := getPortConfigMap(g.logger)
	if err != nil {
		return 0, err
	}
	reservedPortRange := ""
	if rpr, ok := foundConfigMap.Annotations[configuration.GetConfig().ReservedPortRangesAnnotation]; ok {
		reservedPortRange = rpr
	}
	prs, err := g.parsePortRanges(foundConfigMap.Annotations[beginPortKey], reservedPortRange)
	if err != nil {
		return 0, err
	}
	beginPort, endPort := prs.GetMinAndMaxPort()

	nextClusterScopePort, ok := foundConfigMap.Data[cmKey]
	if !ok {
		nextClusterScopePort = strconv.Itoa(beginPort)
	}
	p, err := strconv.Atoi(nextClusterScopePort)
	if err != nil {
		nextClusterScopePort = strconv.Itoa(beginPort)
		p = beginPort
	}
	nextPort := p

	usedPort, err := g.getAllNodeUsedPort()
	if err != nil {
		g.logger.Error(err, "get the port used info error")
		return 0, err
	}

	if nextPort >= endPort {
		// 端口用完，从头再重新检查可用的端口。
		nextPort = beginPort
	}

	selectedPort := false
	for i := nextPort; i < endPort; i++ {
		nextPort += 1
		// 如果port是预留跳过
		if !prs.Contains(nextPort) {
			g.logger.Info(fmt.Sprintf("get the port: %d, ReservedPortRanges has this skip", nextPort))
			continue
		}
		if usedPort != nil {
			isUsedPort := usedPort.Contains(int32(nextPort))
			if isUsedPort {
				continue
			} else {
				selectedPort = true
				break
			}
		}
	}

	if !selectedPort {
		return 0, fmt.Errorf("all port are used, wating for free port, used port : %v", nextPort)
	} else {
		g.logger.Info(fmt.Sprintf("get the port: %d", nextPort))
	}

	strNextPort := strconv.Itoa(nextPort)

	foundConfigMap.Data[cmKey] = strNextPort

	if err := manager.GetSyncClient().Update(context.TODO(), foundConfigMap); err != nil {
		g.logger.Error(err, fmt.Sprintf("can not update %s", cmKey))
		return 0, errors.New("CAN_NOT_UPDATE")
	}
	return nextPort, nil
}

func (g *PortGenerator) getAllNodeUsedPort() (*GoSet, error) {
	nodeList := &corev1.NodeList{}
	if err := manager.GetSyncClient().List(context.TODO(), nodeList, &client.ListOptions{}); err != nil {
		g.logger.Error(err, "get node list error")
		return nil, err
	}
	return g.getAllUsedPort(999999, nodeList)
}

func getPortConfigMap(logger logr.Logger) (*corev1.ConfigMap, error) {
	return k8sutil.GetConfigMap(configuration.GetConfig().PortRangeCmName, define.PortRangeCmNamespace, logger)
}

func (g *PortGenerator) parsePortRanges(portRange, reservedPortRanges string) (*PortRanges, error) {
	prs := &PortRanges{}
	err := prs.Set(portRange)
	if err != nil {
		return nil, err
	}
	if reservedPortRanges == "" {
		return prs, nil
	}
	err = prs.SetReservedPortRanges(reservedPortRanges)
	if err != nil {
		return nil, err
	}
	return prs, nil
}

func (g *PortGenerator) getAllUsedPort(cnt uint64, nodesList *corev1.NodeList) (*GoSet, error) {
	if nodesList == nil || nodesList.Items == nil {
		return nil, nil
	}

	portSet := NewGoSet()
	for _, node := range nodesList.Items {
		way := "configmap"
		usedPort, err := g.getUsedTcpPortByConfigmap(node.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				g.logger.Info(fmt.Sprintf("%d, get node %s port usage configmap is not found, try by ssh", cnt, node.Name))
				way = "ssh"
				usedPort, err = g.getUsedTcpPort(cnt, node.Name)
				if err != nil {
					g.logger.Error(err, fmt.Sprintf("%d, get the node %s used port error", cnt, node.Name))
					continue
				}
			} else {
				g.logger.Error(err, fmt.Sprintf("%d, get the node %s used port by configmap error", cnt, node.Name))
				continue
			}
		}
		if usedPort != nil {
			g.logger.Info(fmt.Sprintf("%d, get %s used Port by %s: %v ", cnt, node.Name, way, usedPort.GetAll()))
			portSet.Add(*usedPort.GetAll()...)
		}
	}

	return portSet, nil
}

func (g *PortGenerator) getUsedTcpPortByConfigmap(host string) (*GoSet, error) {
	cm, err := k8sutil.GetConfigMap(fmt.Sprintf(define.PortUsageCmName, host), define.PortUsageCmNamespace, g.logger)
	if err != nil {
		return nil, err
	}

	goSet := NewGoSet()

	for key, _ := range cm.Data {
		port := strings.TrimSpace(key)
		intPort, err := strconv.Atoi(port)
		if err != nil {
			continue
		}
		if intPort < 1 || intPort > 65535 {
			continue
		}
		goSet.Add(int32(intPort))
	}

	return goSet, nil
}

func (g *PortGenerator) getUsedTcpPort(cnt uint64, host string) (*GoSet, error) {
	getPortCmd := "netstat -tunpa|egrep \"tcp|udp\"|awk '{print $4}'"

	out, errInfo, err := ssh.RunSSHNoPwdCMD(g.logger, getPortCmd, host)
	if err != nil || errInfo != "" && !strings.Contains(errInfo, "") {
		if err.Error() == "exit status 1" && out == "" {
			infoTxt := fmt.Sprintf("%d, GetUsedPort[%s] is nil: cmd: [%s], out:[%s] err:[%s](%v)", cnt, host, getPortCmd, string(out), errInfo, err)
			g.logger.Info(infoTxt)
			// grep 没有拿到结果
			return nil, nil
		}
		errTxt := fmt.Sprintf("%d, GetUsedPort[%s] is failed: cmd: [%s], out:[%s] err:[%s]", cnt, host, getPortCmd, string(out), errInfo)
		g.logger.Error(err, errTxt)
		return nil, fmt.Errorf(errTxt)
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")

	goSet := NewGoSet()

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		addressPart := strings.Split(line, ":")
		if addressPart == nil || len(addressPart) < 1 {
			continue
		}
		lenStr := len(addressPart)
		strPort := addressPart[lenStr-1]
		intPort, err := strconv.Atoi(strPort)
		if err != nil {
			continue
		}
		if intPort < 1 || intPort > 65535 {
			continue
		}
		goSet.Add(int32(intPort))
	}
	return goSet, nil
}
