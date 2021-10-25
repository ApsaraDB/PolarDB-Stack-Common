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

package domain

import (
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

func NewEngineClasses(engineType EngineType, classQuery IClassQuery) (result *EngineClasses, err error) {
	if engineType == "" {
		return nil, errors.New("engineType is empty!")
	}
	if classQuery == nil {
		return nil, errors.New("classQuery is nil!")
	}
	result = &EngineClasses{
		EngineType: engineType,
		classQuery: classQuery,
	}
	return result, nil
}

type EngineClasses struct {
	EngineType EngineType
	classQuery IClassQuery
	classList  []*EngineClass
}

type EngineClass struct {
	Name          string
	ClassKey      string
	CpuRequest    *resource.Quantity
	CpuLimit      *resource.Quantity
	MemoryRequest *resource.Quantity
	MemoryLimit   *resource.Quantity
	HugePageLimit *resource.Quantity
	MemoryShow    *resource.Quantity
	StorageMin    *resource.Quantity
	StorageMax    *resource.Quantity
	Resource      map[string]*InstanceResource
}

func (c *EngineClasses) GetClassList() (list []*EngineClass, err error) {
	if c.classList == nil || len(c.classList) == 0 {
		c.classList, err = c.classQuery.GetClasses(c.EngineType, "")
		if err != nil {
			return
		}
	}
	list = c.classList
	return
}

func (c *EngineClasses) GetClass(classKey string) (item *EngineClass, err error) {
	var list []*EngineClass
	if c.classList == nil || len(c.classList) == 0 {
		list, err = c.classQuery.GetClasses(c.EngineType, classKey)
		if err != nil {
			return
		}
	} else {
		list = c.classList
	}

	for _, class := range list {
		if class.ClassKey == classKey {
			return class, nil
		}
	}

	return nil, errors.New(fmt.Sprintf("GetClass err: %s not found", classKey))
}
