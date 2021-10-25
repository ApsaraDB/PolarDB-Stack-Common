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

package k8sutil

//func TestGetKubeClient(t *testing.T) {
//	//clientset, config, err := GetKubeClient()
//	//klog.Info(clientset, config, err)
//	//
//	//cs, err := GetStandbyClientSet()
//	//sblList, err := cs.StandbyV1beta1().StandbyLinks("").List(metav1.ListOptions{})
//	//klog.Info(sblList)
//	Convey("test", t, func() {
//		Convey("test case 1", func() {
//			test.PatchClient(t, func(container *test.FakeContainer) {
//				standbyLink, err := container.StandbyFakeClientset.StandbyV1beta1().StandbyLinks("").Get("standbylink-sample", v1.GetOptions{})
//				klog.Info(standbyLink.Name)
//				So(err, ShouldBeNil)
//			})
//			monkey.Patch(GetKubeClient, func() (*kubernetes.Clientset, *rest.Config, error) {
//				return &kubernetes.Clientset{}, &rest.Config{}, nil
//			})
//			defer monkey.UnpatchAll()
//			//GetStandbyClientSet()
//		})
//	})
//}
