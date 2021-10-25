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
	"sync"
	"time"

	"k8s.io/client-go/tools/cache"
)

var (
	cacheOnce     sync.Once
	cacheProvider *CacheProvider
	expireTime    = 20 * time.Minute
)

func GetCacheProvider() *CacheProvider {
	cacheOnce.Do(func() {
		if cacheProvider == nil {
			var store = cache.NewTTLStore(func(obj interface{}) (string, error) {
				cacheObj := obj.(cachedObject)
				return cacheObj.key, nil
			}, expireTime)
			cacheProvider = &CacheProvider{store}
		}
	})
	return cacheProvider
}

type cachedObject struct {
	key   string
	value interface{}
}

func createCachedObject(key string, value interface{}) cachedObject {
	return cachedObject{
		key:   key,
		value: value,
	}
}

type CacheProvider struct {
	store cache.Store
}

func (c *CacheProvider) Add(key string, value interface{}) error {
	return c.store.Add(createCachedObject(key, value))
}

func (c *CacheProvider) Get(key string) (value interface{}, exists bool, err error) {
	var result interface{}
	result, exists, err = c.store.GetByKey(key)
	if exists && result != nil {
		value = result.(cachedObject).value
	}
	return
}

func (c *CacheProvider) Update(key string, value interface{}) error {
	return c.store.Update(createCachedObject(key, value))
}

func (c *CacheProvider) Delete(key string) error {
	return c.store.Delete(createCachedObject(key, ""))
}
