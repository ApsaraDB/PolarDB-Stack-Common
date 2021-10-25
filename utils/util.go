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

package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/emicklei/go-restful"
	"github.com/pkg/errors"
)

func MapMerge(m1 map[string]interface{}, m2 map[string]interface{}) map[string]interface{} {
	if m2 == nil {
		return m1
	}
	if m1 == nil {
		return m2
	}
	nm := map[string]interface{}{}
	for k, v := range m2 {
		nm[k] = v
	}
	for k, v := range m1 {
		nm[k] = v
	}
	return nm
}

type ResponseWithMsg struct {
	Status string                 `json:"status"`
	Msg    map[string]interface{} `json:"msg"`
}

func ResponseSucInitFactory() *ResponseWithMsg {
	return &ResponseWithMsg{Status: "ok", Msg: map[string]interface{}{}}
}

type ResponseNilMsg struct {
	Status string `json:"status"`
}

var (
	SuccessResponse = ResponseNilMsg{Status: "ok"}
)

func RandPassword() string {
	return RandString(15)
}

func RandString(n int) string {
	var letterRunes = []rune("1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func MaxIntSlice(v []int) int {
	var m int
	for i, e := range v {
		if i == 0 || e > m {
			m = e
		}
	}
	return m
}

func ReadHttpPostBody(logger logr.Logger, request *restful.Request, response *restful.Response) ([]byte, error) {
	var reader io.Reader = request.Request.Body
	maxFormSize := int64(1<<63 - 1)
	maxFormSizeReal := int64(10 << 20) // 10 MB is a lot of text.
	reader = io.LimitReader(reader, maxFormSizeReal+1)
	b, e := ioutil.ReadAll(reader)
	if e != nil {
		logger.Error(e, "get post msg fail")
		response.WriteErrorString(http.StatusInternalServerError, e.Error())
		return nil, e
	}
	if int64(len(b)) > maxFormSize {
		err := errors.New("http: POST too large")
		response.WriteErrorString(http.StatusInternalServerError, err.Error())
		return nil, e
	}
	return b, nil
}

func ParseInput(logger logr.Logger, request *restful.Request, response *restful.Response, params ...string) ([]string, error) {
	body, err := ReadHttpPostBody(logger, request, response)
	if err != nil {
		return nil, err
	}
	var input map[string]string
	err = json.Unmarshal(body, &input)
	if err != nil {
		response.WriteErrorString(http.StatusBadRequest, err.Error())
		return nil, err
	}

	var values []string
	for _, param := range params {
		if paramValue, ok := input[param]; !ok {
			response.WriteErrorString(http.StatusBadRequest, fmt.Sprintf("please input param: %v", param))
			return nil, errors.Errorf("input param: %v not exists", param)
		} else {
			values = append(values, paramValue)
		}
	}
	return values, nil
}

func MapMergeString(m1 map[string]string, m2 map[string]string) map[string]string {
	if m2 == nil {
		return m1
	}
	if m1 == nil {
		return m2
	}
	nm := map[string]string{}
	for k, v := range m2 {
		nm[k] = v
	}
	for k, v := range m1 {
		nm[k] = v
	}
	return nm
}

func FormatString(str string) string {
	if strings.Contains(str, "_") {
		str = strings.ReplaceAll(str, "_", "-")
	}
	if strings.Contains(str, ".") {
		str = strings.ReplaceAll(str, ".", "-")
	}
	return str
}

func CheckAddressSSHLived(address string, timeSecond time.Duration, logger logr.Logger) bool {
	isLived, _ := CheckAddressByTcp(address, 22, timeSecond, logger)
	return isLived
}

func CheckAddressByTcp(address string, port int, timeout time.Duration, logger logr.Logger) (bool, error) {
	adr := fmt.Sprintf("%s:%d", address, port)
	conn, err := net.DialTimeout("tcp", adr, timeout)
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()
	if err != nil {
		logger.Error(err, fmt.Sprintf("dial tcp address: %s is err status", adr))
		return false, fmt.Errorf("dial tcp address:%s is err status:[%v]", adr, err)
	}
	if conn != nil {
		return true, nil
	}
	logger.Error(errors.New(fmt.Sprintf("dial tcp address: %s is unknown status", adr)), "")
	return false, fmt.Errorf("dial tcp address: %s is unknown status", adr)
}
