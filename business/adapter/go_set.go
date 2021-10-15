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

import "sync"

type GoSet struct {
	bufContainer *map[int32]bool
	locker       sync.Mutex
}

func NewGoSet() *GoSet {
	buf := map[int32]bool{}
	return &GoSet{
		bufContainer: &buf,
		locker:       sync.Mutex{},
	}
}

func (set *GoSet) Add(val ...int32) bool {
	set.locker.Lock()
	defer set.locker.Unlock()

	for _, v := range val {
		_, ok := (*set.bufContainer)[v]
		if ok {
			continue
		}
		(*set.bufContainer)[v] = true
	}

	return true
}

func (set *GoSet) Contains(val ...int32) bool {
	set.locker.Lock()
	defer set.locker.Unlock()

	for _, v := range val {
		_, ok := (*set.bufContainer)[v]
		if !ok {
			return false
		}
	}
	return true
}

func (set *GoSet) GetAll() *[]int32 {
	set.locker.Lock()
	defer set.locker.Unlock()

	var res []int32

	for k := range *set.bufContainer {
		res = append(res, k)
	}
	return &res

}

func (set *GoSet) ClearAll() *[]int32 {
	set.locker.Lock()
	defer set.locker.Unlock()

	var res []int32
	for k := range *set.bufContainer {
		res = append(res, k)
		delete(*set.bufContainer, k)
	}
	return &res
}

func (set *GoSet) Remove(val ...int32) *[]int32 {
	set.locker.Lock()
	defer set.locker.Unlock()

	var res []int32
	for _, k := range val {
		_, ok := (*set.bufContainer)[k]
		if !ok {
			continue
		}
		res = append(res, k)
		delete(*set.bufContainer, k)
	}
	return &res
}
