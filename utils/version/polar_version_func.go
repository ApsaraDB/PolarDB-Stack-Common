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


package version

import (
	"fmt"
	"io/ioutil"
	"k8s.io/klog"
	"os"
	"path"
	"strings"
	"time"
)

const DefaultVersionDir = "/var/lib/polarstack"

func PrintVersionInfo(gitBranch, gitCommitId, gitCommitRepo, gitCommitDate, buildUser, buildHost, podId, containerId string) {
	klog.V(1).Info("------------------------------Print Version------------------------------")
	klog.V(1).Infof("| branch:%v  ", gitBranch)
	klog.V(1).Infof("| commitId:%v ", gitCommitId)
	klog.V(1).Infof("| repo %v", gitCommitRepo)
	klog.V(1).Infof("| commitDate/buildDate %v ", gitCommitDate)
	klog.V(1).Infof("| buildUser: %v, host=%v", buildUser, buildHost)
	klog.V(1).Infof("| podId: %v, containerId=%v", podId, containerId)
	klog.V(1).Info("------------------------------Print Version------------------------------")
}

func WriteVersionInfoToLocal(versionFilePath, gitBranch, gitCommitId, gitCommitRepo, gitCommitDate, buildUser, buildHost string) {
	podId, containerId, err := getSelfContainerInfo()
	if err != nil {
		klog.V(5).Infof("GetSelfContainerInfo err: %v, use default PodId , containerId is nil", err)
		//
	}
	PrintVersionInfo(gitBranch, gitCommitId, gitCommitRepo, gitCommitDate, buildUser, buildHost, podId, containerId)
	versionInfo := fmt.Sprintf("branch:  %s \ncommitId:  %s \nrepo:  %s \ncommitDate/buildDate:  %s \nbuildUser:  %s \nbuildHost:  %s \n  --------- \nstartTime: %v \nPid:%v\n", gitBranch, gitCommitId, gitCommitRepo, gitCommitDate, buildUser, buildHost, time.Now(), os.Getpid())
	runInfo := fmt.Sprintf("PodId: %s \nContainerId: %s\nPidNs:%s\n", podId, containerId, getNs())
	data := []byte(versionInfo + runInfo)
	wFile := parseDirPath(versionFilePath)
	if wFile == "" {
		klog.Errorf("write polardb version err: version dir check error , may be dir is a file")
		return
	}
	wErr := ioutil.WriteFile(wFile, data, 0644)
	if wErr != nil {
		klog.Errorf("write polardb version err:%v", wErr)
	}
}

func parseDirPath(versionFilePath string) string {
	dir, f := path.Split(versionFilePath)
	if dir == "" {
		dir = DefaultVersionDir
	}
	fx, er := os.Stat(dir)
	if er != nil {
		if os.IsNotExist(er) {
			err := os.MkdirAll(dir, 0777)
			if err != nil {
				//
			}
		}
	} else {
		if !fx.IsDir() {
			return ""
		}
	}
	return path.Join(dir, f)

}

func getNs() string {
	ns, err := os.Readlink("/proc/self/ns/pid")
	if err != nil {
		return ""
	}
	return ns
}

func getSelfContainerInfo() (podId, containerId string, err error) {
	contents, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", "", err
	}
	/*
		sample data
		11:devices:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		10:hugetlb:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		9:perf_event:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		8:cpuset:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		7:cpuacct,cpu:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		6:pids:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		5:freezer:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		4:memory:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		3:net_prio,net_cls:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		2:blkio:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
		1:name=systemd:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
	*/
	//contents := []byte("11:devices:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t10:hugetlb:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t9:perf_event:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t8:cpuset:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t7:cpuacct,cpu:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t6:pids:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t5:freezer:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t4:memory:/kubepods.slice/kubepods-pod  aaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t3:net_prio,net_cls:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t2:blkio:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope\n" +
	//	"\t1:name=systemd:/kubepods.slice/kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice/docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope")
	lines := strings.Split(string(contents), "\n")

	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(parts) < 3 {
			continue
		}
		subSystem := strings.TrimSpace(parts[1])
		if !strings.Contains(subSystem, "memory") {
			// only use memory cgroup path
			continue
		}

		cgroupPath := strings.TrimSpace(parts[2])
		if cgroupPath == "" {
			continue
		}
		containerId = parseContainerId(cgroupPath)
		podId = parsePodId(cgroupPath)
		return

	}
	return "", "", fmt.Errorf("cgroup memory not found")
}

func parseContainerId(path string) string {
	parts := strings.Split(path, "/")
	lenParts := len(parts)
	if lenParts < 1 {
		return ""
	}
	dockerPath := ""
	for i := lenParts - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p == "" {
			continue
		}
		dockerPath = p
		break
	}
	if dockerPath == "" {
		return ""
	}
	//sample : docker-4786e379125523639583e35ee57c32d80ac50c7fdca4df1c1d4c80b0562e044f.scope
	if strings.HasSuffix(dockerPath, ".scope") && strings.HasPrefix(dockerPath, "docker-") {
		return strings.TrimSpace(dockerPath[7 : len(dockerPath)-6])
	}
	if strings.HasSuffix(dockerPath, ".scope") {
		return strings.TrimSpace(dockerPath[:len(dockerPath)-6])
	}
	if strings.HasPrefix(dockerPath, "docker-") {
		return strings.TrimSpace(dockerPath[7:])
	}
	return dockerPath
}

func parsePodId(path string) string {
	parts := strings.Split(path, "/")
	lenParts := len(parts)
	if lenParts < 2 {
		return ""
	}
	PodPath := ""
	offSet := 0
	for i := lenParts - 2; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "kubepods-pod") {
			PodPath = p
			offSet = 12
			break
		}
		if strings.HasPrefix(p, "kubepods-burstable-pod") {
			PodPath = p
			offSet = 22
			break
		}
		if strings.HasPrefix(p, "kubepods-besteffort-pod") {
			PodPath = p
			offSet = 23
			break
		}
	}
	if PodPath == "" {
		return ""
	}
	//sample : kubepods-podaaaee9a5_d4d6_11eb_851d_50af732f4b6f.slice
	if strings.HasSuffix(PodPath, ".slice") {
		return strings.ReplaceAll(PodPath[offSet:len(PodPath)-6], "_", "-")
	}
	return strings.ReplaceAll(PodPath[offSet:], "_", "-")
}
