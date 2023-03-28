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
	"testing"
	"time"

	b64 "encoding/base64"

	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/fireflycons/ecr-secret-operator/internal/aws"
	"github.com/fireflycons/ecr-secret-operator/internal/clock"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type errorECRAuthentication struct{}

func (m *errorECRAuthentication) SetCredentials(creds *aws.Credentials, region string) error {
	return fmt.Errorf("Error")
}

func (m *errorECRAuthentication) GetAuthorizationToken() (*ecr.AuthorizationData, error) {
	return nil, fmt.Errorf("Error")
}

func newErrorAuthentication() aws.ECRAuthentication {

	return &errorECRAuthentication{}
}

func makeUid(payload []byte, expiry string, lifetime string) uuid.UUID {
	hash := md5.Sum(append(append(payload, []byte(expiry)...), []byte(lifetime)...))
	uid, _ := uuid.FromBytes(hash[:])
	return uid
}

func TestSecretExtensions(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "KubeSecret Extensions Suite")
}

func prepareUpdateSecret(secret *v1.Secret) error {
	tclock := clock.TestClock{}
	tclock.Set(clock.MustParseTime(aws.TEST_EXPIRY).Add(-clock.MustParseDuration(aws.VALID_LIFETIME)))
	mockAuth := aws.NewMockAuthentication()

	return UpdateSecret(&mockAuth, secret, tclock)
}

var _ = Describe("Kube Secret", func() {

	var secret *v1.Secret

	var payload = []byte(
		fmt.Sprintf(`{"auths":{"%s":{"auth":"%s"}}}`,
			aws.TEST_REGISTRY,
			aws.TEST_AUTH_DATA))

	var payloadEncoded = []byte(b64.StdEncoding.EncodeToString(payload))
	var invalidExpiry = "2023-13-01T00:00:00Z"

	BeforeEach(func() {
		secret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Type: v1.SecretTypeDockerConfigJson,
		}
	})

	Context("UpdateSecret", func() {

		It("Should update the secret with valid expiry time", func() {
			_ = prepareUpdateSecret(secret)
			Expect(secret.Annotations[ANNOTATION_EXPIRES]).To(Equal(aws.TEST_EXPIRY))
		})

		It("Should update the secret with valid lifetime", func() {
			_ = prepareUpdateSecret(secret)
			Expect(secret.Annotations[ANNOTATION_LIFETIME]).To(Equal(aws.VALID_LIFETIME))
		})

		It("Should set correct UUID", func() {
			_ = prepareUpdateSecret(secret)
			Expect(uuid.MustParse(secret.Annotations[ANNOTATION_UID])).To(Equal(makeUid(payload, aws.TEST_EXPIRY, aws.VALID_LIFETIME)))
		})

		It("Should error if error returned by AWS", func() {
			mockAuth := newErrorAuthentication()
			clock := clock.TestClock{}
			err := UpdateSecret(&mockAuth, secret, clock)

			Expect(err).To(HaveOccurred())
		})

	})

	Context("GetSecretData", func() {

		It("Should get valid expiry time", func() {
			tclock := clock.TestClock{}
			tclock.Set(clock.MustParseTime(aws.TEST_EXPIRY).Add(-clock.MustParseDuration(aws.VALID_LIFETIME)))
			mockAuth := aws.NewMockAuthentication()

			a, _, _ := GetSecretData(&mockAuth, tclock)

			Expect(a[ANNOTATION_EXPIRES]).To(Equal(aws.TEST_EXPIRY))
		})

		It("Should get valid lifetime", func() {
			tclock := clock.TestClock{}
			tclock.Set(clock.MustParseTime(aws.TEST_EXPIRY).Add(-clock.MustParseDuration(aws.VALID_LIFETIME)))
			mockAuth := aws.NewMockAuthentication()

			a, _, _ := GetSecretData(&mockAuth, tclock)

			Expect(a[ANNOTATION_LIFETIME]).To(Equal(aws.VALID_LIFETIME))
		})

		It("Should get nil UUID", func() {
			tclock := clock.TestClock{}
			tclock.Set(clock.MustParseTime(aws.TEST_EXPIRY).Add(-clock.MustParseDuration(aws.VALID_LIFETIME)))
			mockAuth := aws.NewMockAuthentication()

			a, _, _ := GetSecretData(&mockAuth, tclock)

			Expect(a[ANNOTATION_UID]).To(Equal(fmt.Sprintf("%v", uuid.Nil)))
		})

		It("Should get correct auth data payload", func() {
			tclock := clock.TestClock{}
			tclock.Set(clock.MustParseTime(aws.TEST_EXPIRY).Add(-clock.MustParseDuration(aws.VALID_LIFETIME)))
			mockAuth := aws.NewMockAuthentication()

			_, d, _ := GetSecretData(&mockAuth, tclock)

			Expect(d[".dockerconfigjson"]).To(Equal(payload))
		})

		It("Should error if error returned by AWS", func() {
			mockAuth := newErrorAuthentication()
			tclock := clock.TestClock{}
			_, _, err := GetSecretData(&mockAuth, tclock)

			Expect(err).To(HaveOccurred())
		})
	})

	Context("GetSecretUuid", func() {

		It("Returns empty UUID if secret data is missing", func() {

			Expect(GetSecretUuid(secret)).To(Equal(uuid.Nil))
		})

		It("Returns empty UUID if expires annotation is missing", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			Expect(GetSecretUuid(secret)).To(Equal(uuid.Nil))
		})

		It("Returns empty UUID if lifetime annotation is missing", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			secret.Annotations = map[string]string{ANNOTATION_EXPIRES: aws.TEST_EXPIRY}
			Expect(GetSecretUuid(secret)).To(Equal(uuid.Nil))
		})

		It("Computes expected UUID", func() {
			expected := makeUid(payloadEncoded, aws.TEST_EXPIRY, aws.VALID_LIFETIME)

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			secret.Annotations = map[string]string{
				ANNOTATION_EXPIRES:  aws.TEST_EXPIRY,
				ANNOTATION_LIFETIME: aws.VALID_LIFETIME,
			}

			Expect(GetSecretUuid(secret)).To(Equal(expected))
		})

	})

	Context("Secret Expiry", func() {

		It("Does not need renewal if expires annotation is missing", func() {

			now, _ := time.Parse(time.RFC3339, "2023-03-01T12:00:00Z")
			tclock := clock.TestClock{}
			tclock.Set(now)

			Expect(IsExpired(secret, time.Duration(0), tclock)).To(BeFalse())
		})

		It("Needs renewal when expires is an illegal timestamp", func() {

			now, _ := time.Parse(time.RFC3339, "2023-03-01T12:00:01Z")
			tclock := clock.TestClock{}
			tclock.Set(now)
			maxAge := time.Hour * 4
			secret.ObjectMeta.Annotations = map[string]string{ANNOTATION_EXPIRES: "2023-13-01T20:00:00Z", ANNOTATION_LIFETIME: "12h"} // Erroneous date

			// Needs an owner reference to be treated as valid.
			secret.OwnerReferences = []metav1.OwnerReference{}
			Expect(IsExpired(secret, maxAge, tclock)).To(BeTrue())
		})

		It("Does not need renewal if lifetime annotation is missing", func() {

			now, _ := time.Parse(time.RFC3339, "2023-03-01T12:00:00Z")
			tclock := clock.TestClock{}
			tclock.Set(now)

			secret.ObjectMeta.Annotations = map[string]string{ANNOTATION_EXPIRES: "2023-01-01T20:00:00Z"}
			Expect(IsExpired(secret, time.Duration(0), tclock)).To(BeTrue())
		})

		It("Needs renewal when lifetime is an illegal duration", func() {

			now, _ := time.Parse(time.RFC3339, "2023-03-01T12:00:01Z")
			tclock := clock.TestClock{}
			tclock.Set(now)
			maxAge := time.Hour * 4
			secret.ObjectMeta.Annotations = map[string]string{ANNOTATION_EXPIRES: "2023-01-01T20:00:00Z", ANNOTATION_LIFETIME: "12x"} // Erroneous duration

			// Needs an owner reference to be treated as valid.
			secret.OwnerReferences = []metav1.OwnerReference{}
			Expect(IsExpired(secret, maxAge, tclock)).To(BeTrue())
		})

		It("Does not need renewal if age is one second younger than max age", func() {

			now, _ := time.Parse(time.RFC3339, "2023-03-01T11:59:59Z")
			tclock := clock.TestClock{}
			tclock.Set(now)
			maxAge := time.Hour * 4
			secret.ObjectMeta.Annotations = map[string]string{ANNOTATION_EXPIRES: "2023-03-01T20:00:00Z", ANNOTATION_LIFETIME: "12h"}

			// Needs an owner reference to be treated as valid.
			secret.OwnerReferences = []metav1.OwnerReference{}
			Expect(IsExpired(secret, maxAge, tclock)).To(BeFalse())
		})

		It("Needs renewal if age is one second older than max age", func() {

			now, _ := time.Parse(time.RFC3339, "2023-03-01T12:00:01Z")
			tclock := clock.TestClock{}
			tclock.Set(now)
			maxAge := time.Hour * 4
			secret.ObjectMeta.Annotations = map[string]string{ANNOTATION_EXPIRES: "2023-03-01T20:00:00Z", ANNOTATION_LIFETIME: "12h"} // Would have been created at 08:00

			// Needs an owner reference to be treated as valid.
			secret.OwnerReferences = []metav1.OwnerReference{}
			Expect(IsExpired(secret, maxAge, tclock)).To(BeTrue())
		})
	})

	Context("Secret Drift", func() {

		It("Is changed if secret payload is missing", func() {

			// Secret is empty here as per BeforeEach
			Expect(IsChanged(secret)).To(BeTrue())
		})

		It("Is changed if expires annotation is missing", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}

			Expect(IsChanged(secret)).To(BeTrue())
		})

		It("Is changed if expires annotation is invalid", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			secret.Annotations = map[string]string{ANNOTATION_EXPIRES: invalidExpiry}

			Expect(IsChanged(secret)).To(BeTrue())
		})

		It("Is changed if lifetime annotation is missing", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			secret.Annotations = map[string]string{ANNOTATION_EXPIRES: aws.TEST_EXPIRY}

			Expect(IsChanged(secret)).To(BeTrue())
		})

		It("Is changed if lifetime annotation is invalid", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			secret.Annotations = map[string]string{
				ANNOTATION_EXPIRES:  aws.TEST_EXPIRY,
				ANNOTATION_LIFETIME: "12x",
			}

			Expect(IsChanged(secret)).To(BeTrue())
		})

		It("Is changed if uid annotation is missing", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			secret.Annotations = map[string]string{
				ANNOTATION_EXPIRES:  aws.TEST_EXPIRY,
				ANNOTATION_LIFETIME: "12h",
			}

			Expect(IsChanged(secret)).To(BeTrue())
		})

		It("Is changed if uid annotation is invalid", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			secret.Annotations = map[string]string{
				ANNOTATION_EXPIRES:  aws.TEST_EXPIRY,
				ANNOTATION_LIFETIME: aws.VALID_LIFETIME,
				ANNOTATION_UID:      "not-a-uuid",
			}

			Expect(IsChanged(secret)).To(BeTrue())
		})

		It("Is changed if uid annotation does not match computed uid", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			secret.Annotations = map[string]string{
				ANNOTATION_EXPIRES:  aws.TEST_EXPIRY,
				ANNOTATION_LIFETIME: aws.VALID_LIFETIME,
				ANNOTATION_UID:      "00000000-0000-0000-0000-000000000000",
			}

			Expect(IsChanged(secret)).To(BeTrue())
		})

		It("Is unchanged if uid annotation matches computed uid", func() {

			secret.Data = map[string][]byte{".dockerconfigjson": payloadEncoded}
			secret.Annotations = map[string]string{
				ANNOTATION_EXPIRES:  aws.TEST_EXPIRY,
				ANNOTATION_LIFETIME: aws.VALID_LIFETIME,
				ANNOTATION_UID:      fmt.Sprintf("%v", makeUid(payloadEncoded, aws.TEST_EXPIRY, aws.VALID_LIFETIME)),
			}

			Expect(IsChanged(secret)).To(BeFalse())
		})
	})
})
