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

package config

import (
	"fmt"
	"io"
	"strings"

	"github.com/fireflycons/ecr-secret-operator/internal/aws"
	"github.com/pelletier/go-toml"
)

const (
	ACCESS_KEY = "access_key"
	SECRET_KEY = "secret_key"
)

const (
	ERROR_FMT_MISSING_CREDS = "FATAL: Credentials for account '%s' not present in configuration"
	ERROR_FMT_MISSING_KEY   = "'%s' missing from config"
)

type Configuration map[string]map[string]string

// Load creds for given AWS account from config
func LoadCredentials(config io.Reader, accountId string) (*aws.Credentials, error) {

	var configuration Configuration

	err := toml.NewDecoder(config).Decode(&configuration)

	if err != nil {
		return nil, err
	}

	// Get creds for the account implied in the ECRSecret resource
	creds, ok := configuration[accountId]

	if !ok {
		return nil, fmt.Errorf(ERROR_FMT_MISSING_CREDS, accountId)
	}

	access_key, ok1 := creds[ACCESS_KEY]
	secret_key, ok2 := creds[SECRET_KEY]

	if ok1 && ok2 {
		return &aws.Credentials{AccessKeyID: access_key, SecretAccessKey: secret_key}, nil
	}

	var errors []string

	if !ok1 {
		errors = append(errors, fmt.Sprintf(ERROR_FMT_MISSING_KEY, ACCESS_KEY))
	}

	if !ok2 {
		errors = append(errors, fmt.Sprintf(ERROR_FMT_MISSING_KEY, SECRET_KEY))
	}

	return nil, fmt.Errorf("FATAL: %s", strings.Join(errors, ", "))
}
