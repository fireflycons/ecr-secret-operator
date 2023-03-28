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
	"strings"

	secretsv1beta1 "github.com/fireflycons/ecr-secret-operator/api/v1beta1"
	"github.com/fireflycons/ecr-secret-operator/internal/aws"
	"github.com/fireflycons/ecr-secret-operator/internal/clock"
	"github.com/fireflycons/ecr-secret-operator/internal/ksecret"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Max age of an ECR secret
//var AWS_SECRET_LIFETIME = time.Hour * 12

// Get the name for the Kubernetes docker-registry secret that will contain the ECR auth token
func getKubeSecretName(ecrSecret *secretsv1beta1.ECRSecret) string {

	if len(strings.TrimSpace(ecrSecret.Spec.SecretName)) > 0 {
		return ecrSecret.Spec.SecretName
	}

	return ecrSecret.Name + "-secret"

}

// Build the kube-secret and make it owned by this custom resource.
func constructSecret(r *ECRSecretReconciler, owner *secretsv1beta1.ECRSecret, ecr *aws.ECRAuthentication, clock clock.Clock) (*corev1.Secret, error) {

	annotations, data, err := ksecret.GetSecretData(ecr, clock)

	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        getKubeSecretName(owner),
			Namespace:   owner.Namespace,
			Annotations: annotations,
		},
		Type: "kubernetes.io/dockerconfigjson",
		Data: data,
	}

	if err := ctrl.SetControllerReference(owner, secret, r.Scheme); err != nil {
		return nil, err
	}

	return secret, nil
}
