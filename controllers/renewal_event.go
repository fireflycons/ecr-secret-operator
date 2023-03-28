/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/fireflycons/ecr-secret-operator/api/v1beta1"
	"github.com/fireflycons/ecr-secret-operator/internal/clock"
	"github.com/fireflycons/ecr-secret-operator/internal/ksecret"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type RenewalEvent struct {
	ctx     context.Context
	log     logr.Logger
	client  client.Client
	lock    sync.RWMutex
	maxAge  time.Duration
	secrets chan<- event.GenericEvent
	clock.Clock
}

func CreateRenewalEvent(client client.Client, secrets chan<- event.GenericEvent) RenewalEvent {
	log := ctrl.Log.
		WithName("source").
		WithName(reflect.TypeOf(RenewalEvent{}).Name())
	return RenewalEvent{
		ctx:     context.Background(),
		log:     log,
		client:  client,
		lock:    sync.RWMutex{},
		secrets: secrets,
		Clock:   clock.RealClock{},
	}
}

func (t *RenewalEvent) Run() {

	nextRun := time.Now()
	frequency := time.Minute

	for {
		select {

		case <-t.ctx.Done():
			return

		default:

			if time.Now().After(nextRun) {
				err := t.pollSecrets()

				if err != nil {
					t.log.Error(err, "error polling secrets")
				}

				nextRun = time.Now().Add(frequency)
			}
		}

		time.Sleep(time.Millisecond * 500)
	}
}

func (t *RenewalEvent) pollSecrets() error {

	t.lock.Lock()
	defer t.lock.Unlock()

	t.log.Info("Polling for secrets that require renewal")

	listNs := corev1.NamespaceList{}

	if err := t.client.List(context.Background(), &listNs); err != nil {

		cerr := &cache.ErrCacheNotStarted{}

		if errors.As(err, &cerr) {
			return nil
		}

		t.log.Error(err, "Unable to list namespaces")
		return err
	}

	for _, ns := range listNs.Items {

		namespaceLog := t.log.WithValues("namespace", ns.Name)

		listSecret := corev1.SecretList{}

		if err := t.client.List(context.Background(), &listSecret, &client.ListOptions{Namespace: ns.Name}); err != nil {
			namespaceLog.Error(err, "Unable to list secrets")
			continue
		}

		for _, secret := range listSecret.Items {

			secretsLog := namespaceLog.WithValues("secret", secret.Name)

			if secret.Type != "kubernetes.io/dockerconfigjson" {
				secretsLog.V(5).Info("Not a docker secret")
				continue
			}

			if ksecret.IsExpired(&secret, t.maxAge, t.Clock) {
				secretsLog.V(5).Info("Secret needs renewal")
				owner := secret.OwnerReferences[0]
				ecrSecret := v1beta1.ECRSecret{}
				err := t.client.Get(t.ctx, types.NamespacedName{Name: owner.Name, Namespace: secret.Namespace}, &ecrSecret)

				if err != nil {
					secretsLog.Error(err, "Cannot get owning secret", "ECRSecret", owner.Name)
					continue
				}

				evt := event.GenericEvent{
					Object: &ecrSecret,
				}

				t.secrets <- evt
			}
		}
	}

	t.log.Info("Poll complete")

	return nil
}
