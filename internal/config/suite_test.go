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
	"strings"
	"testing"

	"github.com/fireflycons/ecr-secret-operator/internal/aws"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", func() {
	Context("Configration Load Errors", func() {

		It("Should fail when input is not TOML", func() {
			_, err := LoadCredentials(strings.NewReader("This is not TOML"), "")
			Expect(err).To(And(HaveOccurred(), Satisfy(func(e error) bool {
				return strings.Contains(e.Error(), "was expecting token")
			})))
		})

		It("Should fail when no matching account found in config", func() {
			toml := `[123456789012]
access_key = "AKAIEXAMPLE"
secret_key = "dsfdsfdfEXAMPLE"`
			missingAccount := "999999999999"
			_, err := LoadCredentials(strings.NewReader(toml), missingAccount)
			Expect(err).To(And(HaveOccurred(), Satisfy(func(e error) bool {

				return e.Error() == fmt.Sprintf(ERROR_FMT_MISSING_CREDS, missingAccount)
			})))
		})

		It("Should fail when access_key not found", func() {
			toml := `[123456789012]
accesskey = "AKAIEXAMPLE"
secret_key = "dsfdsfdfEXAMPLE"`
			_, err := LoadCredentials(strings.NewReader(toml), "123456789012")
			Expect(err).To(And(HaveOccurred(), Satisfy(func(e error) bool {
				return strings.Contains(e.Error(), fmt.Sprintf(ERROR_FMT_MISSING_KEY, "access_key"))
			})))
		})

		It("Should fail when secret_key not found", func() {
			toml := `[123456789012]
access_key = "AKAIEXAMPLE"
secretkey = "dsfdsfdfEXAMPLE"`
			_, err := LoadCredentials(strings.NewReader(toml), "123456789012")
			Expect(err).To(And(HaveOccurred(), Satisfy(func(e error) bool {
				return strings.Contains(e.Error(), fmt.Sprintf(ERROR_FMT_MISSING_KEY, "secret_key"))
			})))
		})
	})

	Context("Configuration Load Scenarios", func() {
		toml := `[123456789012]
access_key = "AKAIEXAMPLE1"
secret_key = "secretEXAMPLE1"

[2109878654321]
access_key = "AKAIEXAMPLE2"
secret_key = "secretEXAMPLE2"`

		It("Should load first account config", func() {
			expected := aws.Credentials{
				AccessKeyID:     "AKAIEXAMPLE1",
				SecretAccessKey: "secretEXAMPLE1",
			}
			creds, _ := LoadCredentials(strings.NewReader(toml), "123456789012")

			Expect(*creds).To(Equal(expected))
		})

		It("Should load second account config", func() {
			expected := aws.Credentials{
				AccessKeyID:     "AKAIEXAMPLE2",
				SecretAccessKey: "secretEXAMPLE2",
			}
			creds, _ := LoadCredentials(strings.NewReader(toml), "2109878654321")

			Expect(*creds).To(Equal(expected))
		})
	})
})
