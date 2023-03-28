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

// Abstracts the details of talking to the ECR API to facilitate testing
package aws

import (
	"fmt"

	b64 "encoding/base64"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/fireflycons/ecr-secret-operator/internal/clock"
)

type Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
}

type ECRAuthentication interface {
	GetAuthorizationToken() (*ecr.AuthorizationData, error)
	SetCredentials(creds *Credentials, region string) error
}

type ConcreteECRAuthentication struct {
	Session *session.Session
}

var (
	TEST_REGISTRY  = "123456789012.dkr.ecr.eu-west-1.amazonaws.com"
	TEST_USER      = "jdoe"
	TEST_PASSWORD  = "pasword123"
	TEST_EXPIRY    = "2023-01-01T12:00:00Z"
	TEST_AUTH_DATA = b64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", TEST_USER, TEST_PASSWORD)))
	VALID_LIFETIME = "12h0m0s"
	TEST_NOW       = clock.MustParseTime(TEST_EXPIRY).Add(-clock.MustParseDuration(VALID_LIFETIME))
)

func (a *ConcreteECRAuthentication) GetAuthorizationToken() (*ecr.AuthorizationData, error) {

	result, err := ecr.New(a.Session).GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, err
	}

	return result.AuthorizationData[0], nil
}

func (a *ConcreteECRAuthentication) SetCredentials(creds *Credentials, region string) error {

	awscreds := credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, "")

	var err error

	a.Session, err = session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: awscreds,
	})

	if err != nil {
		return err
	}

	return nil

}

func NewECRAuthentication() ECRAuthentication {

	return &ConcreteECRAuthentication{Session: nil}
}

type MockECRAuthentication struct{}

func (m *MockECRAuthentication) GetAuthorizationToken() (*ecr.AuthorizationData, error) {
	expires := clock.MustParseTime(TEST_EXPIRY)
	return &ecr.AuthorizationData{
		ExpiresAt:          &expires,
		AuthorizationToken: &TEST_AUTH_DATA,
		ProxyEndpoint:      &TEST_REGISTRY,
	}, nil
}

func (m *MockECRAuthentication) SetCredentials(creds *Credentials, region string) error {
	return nil
}

func NewMockAuthentication() ECRAuthentication {

	return &MockECRAuthentication{}
}
