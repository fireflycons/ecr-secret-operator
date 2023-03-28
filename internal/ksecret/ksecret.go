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

// Extensions for operating on kube secrets
package ksecret

import (
	"crypto/md5"
	"fmt"
	"time"

	"github.com/fireflycons/ecr-secret-operator/internal/aws"
	"github.com/fireflycons/ecr-secret-operator/internal/clock"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
)

const (
	ANNOTATION_UID      = "secrets.fireflycons.io/uuid"
	ANNOTATION_EXPIRES  = "secrets.fireflycons.io/expires"
	ANNOTATION_LIFETIME = "secrets.fireflycons.io/validity"
)

// Compute a UUID based on a hash of the relevant secret content (expires annotation and auth data)
// that will be used to detect changes.
func GetSecretUuid(secret *corev1.Secret) uuid.UUID {

	data, ok := secret.Data[".dockerconfigjson"]

	if !ok {
		return uuid.Nil
	}

	expires, ok := secret.Annotations[ANNOTATION_EXPIRES]

	if !ok {
		return uuid.Nil
	}

	validity, ok := secret.Annotations[ANNOTATION_LIFETIME]

	if !ok {
		return uuid.Nil
	}

	hash := md5.Sum(append(append(data, []byte(expires)...), []byte(validity)...))

	// Theoretically this can not fail.
	// MD5 hash is always 16 bytes, and the error confition for FromBytes
	// is len(argument) != 16
	uid, _ := uuid.FromBytes(hash[:])

	return uid
}

// Check if the secret should be renewed
// It should be renewed if it has exeited for longer than maxAge
// or if there is any kind of error parsing it
func IsExpired(secret *corev1.Secret, maxAge time.Duration, clock clock.Clock) bool {

	expires, ok := secret.Annotations[ANNOTATION_EXPIRES]

	if !ok {
		// If it doesn't have the expires annotation, it's not one of ours
		return false
	}

	// The actual time AWS will cycle the secret
	expireTime, err := time.Parse(time.RFC3339, expires)

	if err != nil {
		return true
	}

	lifetime, ok := secret.Annotations[ANNOTATION_LIFETIME]

	if !ok {
		// If we can't determine the liftetime, expire it.
		return true
	}

	lifeTime, err := time.ParseDuration(lifetime)

	if err != nil {
		return true
	}

	// The time we want to force a recycle of the secret
	t1 := expireTime.Add(maxAge - lifeTime)
	now := clock.Now()

	return (secret.OwnerReferences != nil && now.After(t1))
}

// Determine if the secret has drifted from desired state by comparing value of uid anntation
// with uuid computed from secret content
func IsChanged(secret *corev1.Secret) bool {

	actualUid := GetSecretUuid(secret)

	if actualUid == uuid.Nil {
		// At least one required property is missing
		return true
	}

	uid, ok := secret.Annotations[ANNOTATION_UID]

	if !ok {
		return true
	}

	statedUid, err := uuid.Parse(uid)

	if err != nil {
		// Error would be from parsing UUID
		// Renew he secret due to invalid metadata
		return true
	}

	return (statedUid != actualUid)
}

// Get the data needed to populate the secret
// This being the annotations and the auth data iself
func GetSecretData(ecr *aws.ECRAuthentication, clock clock.Clock) (map[string]string, map[string][]byte, error) {

	authData, err := (*ecr).GetAuthorizationToken()
	if err != nil {
		return nil, nil, err
	}

	validity := authData.ExpiresAt.Sub(clock.Now()).Round(time.Minute)

	anotations := map[string]string{
		ANNOTATION_EXPIRES:  authData.ExpiresAt.Format(time.RFC3339),
		ANNOTATION_UID:      "00000000-0000-0000-0000-000000000000",
		ANNOTATION_LIFETIME: fmt.Sprintf("%v", validity),
	}

	// Note that we don't base64 encode the payload here. APIServer will do that for us
	data := map[string][]byte{
		".dockerconfigjson": []byte(fmt.Sprintf("{\"auths\":{\"%s\":{\"auth\":\"%s\"}}}", *authData.ProxyEndpoint, *authData.AuthorizationToken)),
	}

	return anotations, data, nil
}

// Update a secret to desired state
func UpdateSecret(ecr *aws.ECRAuthentication, secret *corev1.Secret, clock clock.Clock) error {

	annotations, data, err := GetSecretData(ecr, clock)

	if err != nil {
		return err
	}

	// Update the secret's properties first, before computing UID
	secret.Annotations = annotations
	secret.Data = data

	uid := GetSecretUuid(secret)

	// Now set the UID
	annotations[ANNOTATION_UID] = fmt.Sprintf("%v", uid)
	secret.Annotations = annotations

	return nil
}
