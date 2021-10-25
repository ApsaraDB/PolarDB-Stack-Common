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
	"strconv"

	"github.com/ApsaraDB/PolarDB-Stack-Common/configuration"

	"github.com/ApsaraDB/PolarDB-Stack-Common/business/domain"
	mgr "github.com/ApsaraDB/PolarDB-Stack-Common/manager"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type AccountRepository struct {
	Logger              logr.Logger
	GetKubeResourceFunc GetKubeResourceFunc
}

func (r *AccountRepository) EnsureAccountMeta(cluster *domain.DbClusterBase, account *domain.Account) error {
	secretName := getAccountSecretName(cluster.GetClusterResourceName(), account.Account)
	foundSecret := &corev1.Secret{}
	err := mgr.GetSyncClient().Get(context.TODO(), types.NamespacedName{Namespace: cluster.Namespace, Name: secretName}, foundSecret)
	if err != nil && apierrors.IsNotFound(err) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      secretName,
				Labels: map[string]string{
					configuration.GetConfig().AccountMetaClusterLabelName: cluster.Name,
				},
			},
			Type: corev1.SecretTypeOpaque,
			StringData: map[string]string{
				"Account":         account.Account,
				"Password":        account.Password,
				"Priviledge_type": strconv.Itoa(account.PrivilegedType),
			},
		}

		resource, err := r.GetKubeResourceFunc(cluster.Name, cluster.Namespace, cluster.DbClusterType)
		if err != nil {
			return err
		}

		if err := controllerutil.SetControllerReference(resource, secret, mgr.GetManager().GetScheme()); err != nil {
			return err
		}

		err = mgr.GetSyncClient().Create(context.TODO(), secret)
		if err != nil {
			return errors.Wrap(err, "create secret error")
		}
	} else if err != nil {
		return errors.Wrap(err, "get secret error")
	}
	return nil
}

func (r *AccountRepository) GetAccounts(clusterName, namespace string) (accounts map[string]*domain.Account, err error) {
	labelSelector, err := labels.Parse(configuration.GetConfig().AccountMetaClusterLabelName + "=" + clusterName)
	if err != nil {
		return nil, errors.New("parse label selector error")
	}

	secretList := &corev1.SecretList{}
	if err := mgr.GetSyncClient().List(context.TODO(), secretList, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	}); err != nil {
		return nil, errors.New("get secrets error")
	}

	accounts = make(map[string]*domain.Account)
	for _, secret := range secretList.Items {
		data := secret.Data
		account, ok1 := data["Account"]
		password, ok2 := data["Password"]
		strPrivilegedType, ok3 := data["Priviledge_type"]
		if !ok1 || !ok2 || !ok3 {
			r.Logger.Info("a secret ignored: " + secret.Namespace + secret.Name)
			continue
		}
		privilegedType, err := strconv.Atoi(string(strPrivilegedType))
		if err != nil {
			r.Logger.Info("a secret ignored: " + secret.Namespace + secret.Name)
			continue
		}
		accounts[string(account)] = &domain.Account{
			PrivilegedType: privilegedType,
			Account:        string(account),
			Password:       string(password),
		}
	}
	return accounts, nil
}

func getAccountSecretName(resourceName, account string) string {
	return resourceName + "-" + account
}
