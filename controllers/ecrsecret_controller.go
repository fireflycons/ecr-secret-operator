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
	"fmt"
	"os"
	"strings"
	"time"

	secretsv1beta1 "github.com/fireflycons/ecr-secret-operator/api/v1beta1"
	"github.com/fireflycons/ecr-secret-operator/internal/aws"
	"github.com/fireflycons/ecr-secret-operator/internal/clock"
	"github.com/fireflycons/ecr-secret-operator/internal/config"
	"github.com/fireflycons/ecr-secret-operator/internal/ksecret"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ECRSecretReconciler reconciles a ECRSecret object
type ECRSecretReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	ConfigFile string
	MaxAge     time.Duration
	clock.Clock
	Auth aws.ECRAuthentication
}

//+kubebuilder:rbac:groups=secrets.fireflycons.io,resources=ecrsecrets,verbs="*"
//+kubebuilder:rbac:groups=secrets.fireflycons.io,resources=ecrsecrets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=secrets.fireflycons.io,resources=ecrsecrets/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs="*"
//+kubebuilder:rbac:groups="",resources=secrets/status,verbs=get
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ECRSecret object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ECRSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.V(5).Info("Begin reconciler")

	var (
		emptyResult = ctrl.Result{}
		ecrSecret   secretsv1beta1.ECRSecret
	)

	// Retrieve the custom resource
	if err := r.Get(ctx, req.NamespacedName, &ecrSecret); err != nil {
		// https://stuartleeks.com/posts/kubebuilder-event-filters-part-1-delete/
		if apierrs.IsNotFound(err) {
			// It has just been deleted

			log.Info("Unable to fetch ECRSecret - probably just deleted.")
			return emptyResult, nil
		}

		// A genuine error
		log.Error(err, "unable to fetch ECRSecret")
		return emptyResult, err
	}

	// Determine AWS account ID and region from registy property of spec
	// Not used yet: account ID
	accountId, region := func(registry string) (string, string) {

		arr := strings.Split(registry, ".")
		return arr[0], arr[3]
	}(ecrSecret.Spec.Registry)

	log.V(5).Info("Read ECRSecret", "AccountID", accountId, "Region", region)

	//
	// Handle changes
	//

	// Load config (in case it changed)
	// If we can't load the config correctly, crashloop the container so someone notices
	configStream, err := os.Open(r.ConfigFile)

	if err != nil {
		log.Error(err, fmt.Sprintf("FATAL: Cannot load '%s'.", r.ConfigFile))
		os.Exit(1)
	}

	credentials, err := config.LoadCredentials(configStream, accountId)

	if err != nil {
		// loadCredentials formats the error message
		log.Error(err, err.Error())
		os.Exit(1)
	}

	log.V(5).Info("Loaded AWS credentials", "AccountID", accountId, "AccessKey", credentials.AccessKeyID)

	// AWS session to use for this resource
	err = r.Auth.SetCredentials(credentials, region)

	if err != nil {
		return emptyResult, err
	}

	foundSecret := &corev1.Secret{}

	// Look for existing owned kube secret
	err = r.Get(ctx, types.NamespacedName{Name: getKubeSecretName(&ecrSecret), Namespace: ecrSecret.Namespace}, foundSecret)

	if err != nil && apierrs.IsNotFound(err) {
		// If we get here, need to create a new secret
		var secret *corev1.Secret

		log.V(5).Info("Creating new docker-registry secret", "Name", getKubeSecretName(&ecrSecret))
		secret, err = constructSecret(r, &ecrSecret, &r.Auth, r.Clock)

		if err != nil {
			return emptyResult, err
		}

		id := ksecret.GetSecretUuid(secret)

		secret.Annotations[ksecret.ANNOTATION_UID] = fmt.Sprintf("%v", id)

		if err = r.Create(ctx, secret); err != nil {
			log.Error(err, "unable to create secret for ECRSecret", "ECRSecret", ecrSecret.Name)
			return emptyResult, err
		}

		log.Info("Created new docker-registry secret", "ECRSecret", ecrSecret.Name, "Secret", secret.Name, "uuid", fmt.Sprintf("%v", id))

	} else if err == nil {

		// Some crud operation has happened to the owned secret, or we received a renewal event

		if ksecret.IsChanged(foundSecret) || ksecret.IsExpired(foundSecret, r.MaxAge, r.Clock) {
			// Owned secret has drifted from desired state or has expired
			// Update to required state - effectively regenerate the secret

			if err = ksecret.UpdateSecret(&r.Auth, foundSecret, r.Clock); err == nil {
				log.Info("Updating secret", "secret", foundSecret.Name)
				err = r.Update(ctx, foundSecret)
			}
		}
	}

	return emptyResult, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ECRSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// set up concrete implementations, since we're not in a test
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	if r.Auth == nil {
		r.Auth = aws.NewECRAuthentication()
	}

	// Start a polling loop to look for expiry
	ch := make(chan event.GenericEvent)
	updateEvent := CreateRenewalEvent(mgr.GetClient(), ch)
	go updateEvent.Run()

	return ctrl.NewControllerManagedBy(mgr).
		For(&secretsv1beta1.ECRSecret{}).
		Watches(&source.Channel{Source: ch, DestBufferSize: 1024}, &handler.EnqueueRequestForObject{}).
		Owns(&corev1.Secret{}). // https://github.com/kubernetes-sigs/kubebuilder/blob/master/docs/book/src/reference/watching-resources/testdata/owned-resource/controller.go
		Complete(r)
}
