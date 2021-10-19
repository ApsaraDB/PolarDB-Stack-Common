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


package ssh

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/ApsaraDB/PolarDB-Stack-Common/utils/k8sutil"

	"golang.org/x/crypto/ssh"
)

func PublicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}

func SSHConnect(user, password, host string, port int) (*ssh.Session, *ssh.Client, error) {
	var (
		auth         []ssh.AuthMethod
		addr         string
		clientConfig *ssh.ClientConfig
		client       *ssh.Client
		session      *ssh.Session
		err          error
	)
	// get auth method
	auth = make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(password))

	hostKeyCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return nil
	}
	if password == "" {
		auth = []ssh.AuthMethod{
			PublicKeyFile("/root/.ssh/id_rsa"),
		}
	}
	clientConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		Timeout:         30 * time.Second,
		HostKeyCallback: hostKeyCallback,
	}

	addr = fmt.Sprintf("%s:%d", host, port)

	if client, err = ssh.Dial("tcp", addr, clientConfig); err != nil {
		return nil, nil, err
	}

	// create session
	if session, err = client.NewSession(); err != nil {
		return nil, nil, err
	}

	return session, client, nil
}

// run shell command
func RunSSHNoPwdCMD(logger logr.Logger, cmd string, ipaddr string) (string, string, error) {
	logger = logger.WithValues("cmd", cmd, "ipaddr", ipaddr)
	controllerConf, err := k8sutil.GetControllerConfig(logger)
	if err != nil {
		logger.Error(err, "runSsh command failed, get ssh user and password error.")
		return "", "", err
	}

	connBeginTime := time.Now()

	var stdOut, stdErr bytes.Buffer
	logger.Info("runSsh command")
	session, client, err := SSHConnect(controllerConf.SshUser, controllerConf.SshPassword, ipaddr, 22)
	if err != nil {
		logger.Error(err, fmt.Sprintf("runSsh command failed on build connect."))
		return "", "", err
	}
	defer func() {
		session.Close()
		client.Close()
	}()

	cmdStartTime := time.Now()

	session.Stdout = &stdOut
	session.Stderr = &stdErr

	runErr := session.Run(cmd)
	if runErr != nil {
		logger.Error(runErr, "run cmd error")
	}

	stdOutStr := stdOut.String()
	stdErrStr := stdErr.String()

	stdOutStrWithNoEnter := strings.ReplaceAll(stdOutStr, "\n", "\\n")
	stdErrStrWithNoEnter := strings.ReplaceAll(stdErrStr, "\n", "\\n")

	now := time.Now()

	logger.Info("runSsh finished.",
		"cost", fmt.Sprintf("%v s", now.Sub(cmdStartTime).Seconds()),
		"total", fmt.Sprintf("%v s", now.Sub(connBeginTime).Seconds()),
		"out", stdOutStrWithNoEnter,
		"errOut", stdErrStrWithNoEnter,
		"err", fmt.Sprintf("%v", err),
	)

	return stdOutStr, stdErrStr, nil
}
