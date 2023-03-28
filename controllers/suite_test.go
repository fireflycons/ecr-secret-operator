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
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	secretsv1beta1 "github.com/fireflycons/ecr-secret-operator/api/v1beta1"
	"github.com/fireflycons/ecr-secret-operator/internal/aws"
	"github.com/fireflycons/ecr-secret-operator/internal/clock"
	"github.com/fireflycons/ecr-secret-operator/internal/ksecret"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var k8sManager ctrl.Manager

// Bring the mechanism for creating the manager context here so that we can forcibly cancel it
// https://github.com/kubernetes-sigs/controller-runtime/issues/1571
var onlyOneSignalHandler = make(chan struct{})
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
var cancel context.CancelFunc

func setupSignalHandler() context.Context {
	close(onlyOneSignalHandler) // panics when called twice
	var ctx context.Context
	ctx, cancel = context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = secretsv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	testClock := clock.TestClock{}
	testClock.Set(aws.TEST_NOW)

	err = (&ECRSecretReconciler{
		Client:     k8sManager.GetClient(),
		Scheme:     k8sManager.GetScheme(),
		MaxAge:     time.Hour * 4,
		Clock:      testClock,
		ConfigFile: filepath.Join("..", "config.toml"),
		Auth:       aws.NewMockAuthentication(),
	}).SetupWithManager((k8sManager))
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(setupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

})

var secretName = "test-secret"
var secretNamespace = "default"
var secretLookupKey = types.NamespacedName{Name: secretName, Namespace: secretNamespace}

var _ = Describe("ECR Secret Lifecycle", func() {
	It("Should create secret", func() {

		ctx := context.Background()

		By("By creating a new ECRSecret")
		spec := secretsv1beta1.ECRSecretSpec{
			Registry:   aws.TEST_REGISTRY,
			SecretName: secretName,
		}

		ecrsecret := secretsv1beta1.ECRSecret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "ecrsecrets.secrets.fireflycons.io/v1beta1",
				Kind:       "ECRSecret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
			},
			Spec: spec,
		}

		Expect(k8sClient.Create(ctx, &ecrsecret)).Should(Succeed())

		createdEcrSecret := &secretsv1beta1.ECRSecret{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, secretLookupKey, createdEcrSecret)

			return err == nil
		}, time.Second*5, time.Second).Should(BeTrue())

		By("A kube secret should also be created")

		createdSecret := &v1.Secret{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, secretLookupKey, createdSecret)
			return err == nil
		}, time.Second*5, time.Second).Should(BeTrue())

		By("Checking created secret properties")

		Expect(createdSecret.Name).To(Equal(secretName))
		Expect(createdSecret.Annotations[ksecret.ANNOTATION_EXPIRES]).To(Equal(aws.TEST_EXPIRY))
		Expect(createdSecret.Annotations[ksecret.ANNOTATION_LIFETIME]).To(Equal(aws.VALID_LIFETIME))

		By("Deleting the ECR secret")

		Eventually(func() bool {
			// Force a foregrround delete of dependent ojbect so as not to have to wait for garbage collection
			x := metav1.DeletePropagationForeground
			err := k8sClient.Delete(ctx, &ecrsecret, &client.DeleteOptions{PropagationPolicy: &x})

			return err == nil
		}, time.Second*5, time.Second).Should(BeTrue())

		By("Dependent kube secret should also be deleted")

		deletedSecret := &v1.Secret{}

		Eventually(func() bool {
			err := k8sClient.Get(ctx, secretLookupKey, deletedSecret)
			return err == nil
		}, time.Second*5, time.Second).Should(BeTrue())

	})
})

var _ = Describe("CRD errors", func() {
	invalidRegistry := "docker.io"
	badSecretName := "should-fail-secret"

	It("Should fail if registry does not match pattern for ECR", func() {

		ctx := context.Background()

		By("By creating a new ECRSecret")
		spec := secretsv1beta1.ECRSecretSpec{
			Registry:   invalidRegistry,
			SecretName: badSecretName,
		}

		ecrsecret := secretsv1beta1.ECRSecret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "ecrsecrets.secrets.fireflycons.io/v1beta1",
				Kind:       "ECRSecret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      badSecretName,
				Namespace: secretNamespace,
			},
			Spec: spec,
		}

		err := k8sClient.Create(ctx, &ecrsecret)
		Expect(err).To(HaveOccurred())

	})
})

var _ = Describe("ECRSecret", func() {
	Context("Get Secret Name", func() {
		It("Should return generated name if no specific name provided", func() {
			expected := "test-secret"
			sec := secretsv1beta1.ECRSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: secretsv1beta1.ECRSecretSpec{},
			}

			Expect(getKubeSecretName(&sec)).To(Equal(expected))
		})
		It("Should return specific name if specific name provided", func() {
			expected := "my=secret"
			sec := secretsv1beta1.ECRSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: secretsv1beta1.ECRSecretSpec{
					SecretName: expected,
				},
			}

			Expect(getKubeSecretName(&sec)).To(Equal(expected))
		})
	})
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
