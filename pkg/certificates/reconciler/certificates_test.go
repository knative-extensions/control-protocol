/*
Copyright 2021 The Knative Authors

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

package sample

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakesecretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/filtered/fake"

	filteredFactory "knative.dev/pkg/client/injection/kube/informers/factory/filtered"
	_ "knative.dev/pkg/client/injection/kube/informers/factory/filtered/fake"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	pkgreconciler "knative.dev/pkg/reconciler"
	. "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/system"

	"knative.dev/control-protocol/pkg/certificates"
)

func setupTest(t *testing.T, ctor injection.ControllerConstructor) (context.Context, *controller.Impl) {
	ctx, cf, _ := SetupFakeContextWithCancel(t, func(ctx context.Context) context.Context {
		return filteredFactory.WithSelectors(ctx, "my-ctrl")
	})
	t.Cleanup(cf)

	configMapWatcher := &configmap.ManualWatcher{Namespace: system.Namespace()}
	ctrl := ctor(ctx, configMapWatcher)

	// The Reconciler won't do any work until it becomes the leader.
	if la, ok := ctrl.Reconciler.(pkgreconciler.LeaderAware); ok {
		require.NoError(t, la.Promote(
			pkgreconciler.UniversalBucket(),
			func(pkgreconciler.Bucket, types.NamespacedName) {},
		))
	}
	return ctx, ctrl
}

func TestReconcile(t *testing.T) {
	// The key to use, which for this singleton reconciler doesn't matter (although the
	// namespace matters for namespace validation).
	namespace := system.Namespace()
	caSecretName := "my-ctrl-ca"
	labelName := "my-ctrl"

	caKP, caKey, caCertificate := mustCreateCACert(t, 10*time.Hour)

	wellFormedCaSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      caSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			certificates.SecretCertKey: caKP.CertBytes(),
			certificates.SecretPKKey:   caKP.PrivateKeyBytes(),
		},
	}

	controlPlaneKP := mustCreateControlPlaneCert(t, 10*time.Hour, caKey, caCertificate)

	wellFormedControlPlaneSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "control-plane-ctrl",
			Namespace: namespace,
			Labels: map[string]string{
				labelName: controlPlaneSecretType,
			},
		},
		Data: map[string][]byte{
			certificates.SecretCaCertKey: caKP.CertBytes(),
			certificates.SecretCertKey:   controlPlaneKP.CertBytes(),
			certificates.SecretPKKey:     controlPlaneKP.PrivateKeyBytes(),
		},
	}

	dataPlaneKP := mustCreateDataPlaneCert(t, 10*time.Hour, caKey, caCertificate)

	wellFormedDataPlaneSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-plane-ctrl",
			Namespace: namespace,
			Labels: map[string]string{
				labelName: dataPlaneSecretType,
			},
		},
		Data: map[string][]byte{
			certificates.SecretCaCertKey: caKP.CertBytes(),
			certificates.SecretCertKey:   dataPlaneKP.CertBytes(),
			certificates.SecretPKKey:     dataPlaneKP.PrivateKeyBytes(),
		},
	}

	tests := []struct {
		name                   string
		key                    string
		executeReconcilerTwice bool
		objects                []*corev1.Secret
		asserts                map[string]func(*testing.T, *corev1.Secret)
	}{{
		name:    "well formed secret CA and control plane secret exists",
		key:     namespace + "/control-plane-ctrl",
		objects: []*corev1.Secret{wellFormedCaSecret, wellFormedControlPlaneSecret},
		asserts: map[string]func(*testing.T, *corev1.Secret){
			wellFormedCaSecret.Name: func(t *testing.T, secret *corev1.Secret) {
				require.Equal(t, wellFormedCaSecret, secret)
			},
			wellFormedControlPlaneSecret.Name: func(t *testing.T, secret *corev1.Secret) {
				require.Equal(t, wellFormedControlPlaneSecret, secret)
			},
		},
	}, {
		name:    "well formed secret CA and data plane secret exists",
		key:     namespace + "/data-plane-ctrl",
		objects: []*corev1.Secret{wellFormedCaSecret, wellFormedDataPlaneSecret},
		asserts: map[string]func(*testing.T, *corev1.Secret){
			wellFormedCaSecret.Name: func(t *testing.T, secret *corev1.Secret) {
				require.Equal(t, wellFormedCaSecret, secret)
			},
			wellFormedDataPlaneSecret.Name: func(t *testing.T, secret *corev1.Secret) {
				require.Equal(t, wellFormedDataPlaneSecret, secret)
			},
		},
	}, {
		name:                   "empty CA secret and empty control plane secret",
		key:                    namespace + "/control-plane-ctrl",
		executeReconcilerTwice: true,
		objects: []*corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caSecretName,
				Namespace: namespace,
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane-ctrl",
				Namespace: namespace,
				Labels: map[string]string{
					labelName: controlPlaneSecretType,
				},
			},
		}},
		asserts: map[string]func(*testing.T, *corev1.Secret){
			caSecretName:         validCACert,
			"control-plane-ctrl": validControlPlaneCert,
		},
	}, {
		name:                   "empty CA secret and empty data plane secret",
		key:                    namespace + "/data-plane-ctrl",
		executeReconcilerTwice: true,
		objects: []*corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caSecretName,
				Namespace: namespace,
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-plane-ctrl",
				Namespace: namespace,
				Labels: map[string]string{
					labelName: dataPlaneSecretType,
				},
			},
		}},
		asserts: map[string]func(*testing.T, *corev1.Secret){
			caSecretName:      validCACert,
			"data-plane-ctrl": validControlPlaneCert,
		},
	}, {
		name: "well formed secret CA but empty control plane secret",
		key:  namespace + "/control-plane-ctrl",
		objects: []*corev1.Secret{wellFormedCaSecret, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane-ctrl",
				Namespace: namespace,
				Labels: map[string]string{
					labelName: controlPlaneSecretType,
				},
			},
		}},
		asserts: map[string]func(*testing.T, *corev1.Secret){
			wellFormedCaSecret.Name: func(t *testing.T, secret *corev1.Secret) {
				require.Equal(t, wellFormedCaSecret, secret)
			},
			"control-plane-ctrl": validControlPlaneCert,
		},
	}, {
		name:                   "malformed secret CA and malformed control plane secret",
		key:                    namespace + "/control-plane-ctrl",
		executeReconcilerTwice: true,
		objects: []*corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caSecretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				certificates.SecretCertKey: caKP.CertBytes(),
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane-ctrl",
				Namespace: namespace,
				Labels: map[string]string{
					labelName: controlPlaneSecretType,
				},
			},
			Data: map[string][]byte{
				certificates.SecretCaCertKey: {1, 2, 3},
				certificates.SecretCertKey:   {1, 2, 3},
				certificates.SecretPKKey:     {1, 2, 3},
			},
		}},
		asserts: map[string]func(*testing.T, *corev1.Secret){
			caSecretName:         validCACert,
			"control-plane-ctrl": validControlPlaneCert,
		},
	}, {
		name: "well formed secret CA and malformed control plane secret",
		key:  namespace + "/control-plane-ctrl",
		objects: []*corev1.Secret{wellFormedCaSecret, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane-ctrl",
				Namespace: namespace,
				Labels: map[string]string{
					labelName: controlPlaneSecretType,
				},
			},
		}},
		asserts: map[string]func(*testing.T, *corev1.Secret){
			caSecretName:         validCACert,
			"control-plane-ctrl": validControlPlaneCert,
		},
	}, {
		name: "no CA secret and empty control plane secret",
		key:  namespace + "/control-plane-ctrl",
		objects: []*corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane-ctrl",
				Namespace: namespace,
				Labels: map[string]string{
					labelName: controlPlaneSecretType,
				},
			},
		}},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, ctrl := setupTest(t, NewControllerFactory("my"))

			for _, s := range test.objects {
				_, err := fakekubeclient.Get(ctx).CoreV1().Secrets(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
				require.NoError(t, err)
				require.NoError(t, fakesecretinformer.Get(ctx, labelName).Informer().GetIndexer().Add(s))
			}

			require.NoError(t, ctrl.Reconciler.Reconcile(ctx, test.key))
			if test.executeReconcilerTwice {
				// Update the informers cache
				secrets, _ := fakekubeclient.Get(ctx).CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
				for _, s := range secrets.Items {
					s := (&s).DeepCopy()
					require.NoError(t, fakesecretinformer.Get(ctx, labelName).Informer().GetIndexer().Update(s))
				}
				// Reconcile again
				require.NoError(t, ctrl.Reconciler.Reconcile(ctx, test.key))
				require.NoError(t, ctrl.Reconciler.Reconcile(ctx, test.key))
			}

			for name, asserts := range test.asserts {
				sec, err := fakekubeclient.Get(ctx).CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
				require.NoError(t, err)
				asserts(t, sec)
			}
		})
	}
}

func mustCreateCACert(t *testing.T, expirationInterval time.Duration) (*certificates.KeyPair, *rsa.PrivateKey, *x509.Certificate) {
	kp, err := certificates.CreateCACerts(context.TODO(), expirationInterval)
	require.NoError(t, err)
	pk, cert, err := certificates.ParseCert(kp.CertBytes(), kp.PrivateKeyBytes())
	require.NoError(t, err)
	return kp, cert, pk
}

func mustCreateDataPlaneCert(t *testing.T, expirationInterval time.Duration, caKey *rsa.PrivateKey, caCertificate *x509.Certificate) *certificates.KeyPair {
	kp, err := certificates.CreateDataPlaneCert(context.TODO(), caKey, caCertificate, expirationInterval)
	require.NoError(t, err)
	return kp
}

func mustCreateControlPlaneCert(t *testing.T, expirationInterval time.Duration, caKey *rsa.PrivateKey, caCertificate *x509.Certificate) *certificates.KeyPair {
	kp, err := certificates.CreateControlPlaneCert(context.TODO(), caKey, caCertificate, expirationInterval)
	require.NoError(t, err)
	return kp
}

func validCACert(t *testing.T, secret *corev1.Secret) {
	require.Contains(t, secret.Data, certificates.SecretPKKey)
	require.Contains(t, secret.Data, certificates.SecretCertKey)
	cert, pk, err := certificates.ParseCert(secret.Data[certificates.SecretCertKey], secret.Data[certificates.SecretPKKey])
	require.NotNil(t, cert)
	require.NotNil(t, pk)
	require.NoError(t, err)
}

func validDataPlaneCert(t *testing.T, secret *corev1.Secret) {
	require.Contains(t, secret.Data, certificates.SecretCaCertKey)
	validCACert(t, secret)
}

var validControlPlaneCert = validDataPlaneCert

func TestEmptySecret(t *testing.T) {
	namespace := system.Namespace()
	caSecretName := "my-ctrl-ca"
	labelName := "my-ctrl"

	tests := []struct {
		name                   string
		key                    string
		executeReconcilerTwice bool
		objects                []*corev1.Secret
		asserts                map[string]func(*testing.T, *corev1.Secret)
	}{{
		name:                   "empty CA secret and empty control plane secret",
		key:                    namespace + "/control-plane-ctrl",
		executeReconcilerTwice: true,
		objects: []*corev1.Secret{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caSecretName,
				Namespace: namespace,
				Labels: map[string]string{
					labelName: "",
				},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane-ctrl",
				Namespace: namespace,
				Labels: map[string]string{
					labelName: controlPlaneSecretType,
				},
			},
		}},
		asserts: map[string]func(*testing.T, *corev1.Secret){
			caSecretName:         validCACert,
			"control-plane-ctrl": validControlPlaneCert,
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, ctrl := setupTest(t, NewControllerFactory("my"))

			for _, s := range test.objects {
				_, err := fakekubeclient.Get(ctx).CoreV1().Secrets(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
				require.NoError(t, err)
				require.NoError(t, fakesecretinformer.Get(ctx, labelName).Informer().GetIndexer().Add(s))
			}

			require.NoError(t, ctrl.Reconciler.Reconcile(ctx, test.key))
			if test.executeReconcilerTwice {
				// Update the informers cache
				secrets, _ := fakekubeclient.Get(ctx).CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
				for _, s := range secrets.Items {
					s := (&s).DeepCopy()
					require.NoError(t, fakesecretinformer.Get(ctx, labelName).Informer().GetIndexer().Update(s))
				}
				// Reconcile again
				require.NoError(t, ctrl.Reconciler.Reconcile(ctx, test.key))
				require.NoError(t, ctrl.Reconciler.Reconcile(ctx, test.key))
			}

			for name, asserts := range test.asserts {
				sec, err := fakekubeclient.Get(ctx).CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
				require.NoError(t, err)
				asserts(t, sec)
			}
		})
	}
}
