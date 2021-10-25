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
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	"github.com/ApsaraDB/PolarDB-Stack-Common/test"
	. "github.com/bouk/monkey"
	"github.com/pkg/errors"
	. "github.com/smartystreets/goconvey/convey"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/klogr"
)

func Test_EnsureAccountMeta(t *testing.T) {
	Convey("EnsureAccountMeta", t, func() {
		test.PatchClient(t, func(container *test.FakeContainer) {
			repo := AccountRepository{
				Logger: klogr.New(),
				GetKubeResourceFunc: func(name, namespace string, clusterType domain.DbClusterType) (metav1.Object, error) {
					return nil, nil
				},
			}

			Convey("GetKubeResourceFunc error", func() {
				errMsg := "GetKubeResourceFunc error"
				repo.GetKubeResourceFunc = func(name, namespace string, clusterType domain.DbClusterType) (metav1.Object, error) {
					return nil, errors.New(errMsg)
				}

				cluster := &domain.DbClusterBase{
					Name:       "polar-rwo-50ox133d306",
					Namespace:  "default",
					LogicInsId: "22512",
				}

				testAccount := &domain.Account{
					Account:        "polar",
					Password:       "testPwd",
					PrivilegedType: 0,
				}

				err := repo.EnsureAccountMeta(cluster, testAccount)
				So(err, ShouldNotBeNil)
				So(err.Error() == errMsg, ShouldBeTrue)
			})

			Convey("SetControllerReference error", func() {
				errMsg := "SetControllerReference error"
				guard := Patch(controllerutil.SetControllerReference, func(owner, controlled metav1.Object, scheme *runtime.Scheme) error {
					return errors.New("SetControllerReference error")
				})
				defer guard.Unpatch()
				cluster := &domain.DbClusterBase{
					Name:       "polar-rwo-50ox133d306",
					LogicInsId: "22512",
				}

				testAccount := &domain.Account{
					Account:        "replicator",
					Password:       "testPwd",
					PrivilegedType: 0,
				}

				err := repo.EnsureAccountMeta(cluster, testAccount)
				So(err, ShouldNotBeNil)
				So(err.Error() == errMsg, ShouldBeTrue)
			})

			Convey("ignore when metadata already exists", func() {
				//errMsg := "GetKubeResourceFunc error"
				cluster := &domain.DbClusterBase{
					Name:       "polar-rwo-50ox133d306",
					Namespace:  "default",
					LogicInsId: "22512",
				}

				testAccount := &domain.Account{
					Account:        "replicator",
					Password:       "testPwd",
					PrivilegedType: 0,
				}

				err := repo.EnsureAccountMeta(cluster, testAccount)
				So(err, ShouldBeNil)
			})

			Convey("executed successfully when metadata does not exist", func() {
				cluster := &domain.DbClusterBase{
					Name:       "polar-rwo-50ox133d306",
					Namespace:  "default",
					LogicInsId: "22515",
				}

				testAccount := &domain.Account{
					Account:        "replicator",
					Password:       "testPwd",
					PrivilegedType: 0,
				}

				err := repo.EnsureAccountMeta(cluster, testAccount)
				So(err, ShouldBeNil)
			})
		})
	})
}
