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

package reconciler_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/fake"
	"knative.dev/pkg/injection"

	"knative.dev/control-protocol/pkg/certificates"
	"knative.dev/control-protocol/pkg/reconciler"
)

func TestListerCertificateGetter_GenerateTLSDialer(t *testing.T) {
	name := "my-secret"
	namespace := "my-namespace"

	caKP, err := certificates.CreateCACerts(context.TODO(), 24*time.Hour)
	require.NoError(t, err)
	caCert, caPrivate, err := caKP.Parse()
	require.NoError(t, err)

	kp, err := certificates.CreateControlPlaneCert(context.TODO(), caPrivate, caCert, 24*time.Hour, certificates.DataPlaneNamePrefix+namespace)
	require.NoError(t, err)

	secretData := make(map[string][]byte)
	secretData[certificates.SecretCaCertKey] = caKP.CertBytes()
	secretData[certificates.SecretCertKey] = kp.CertBytes()
	secretData[certificates.SecretPKKey] = kp.PrivateKeyBytes()

	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr bool
	}{{
		name:    "no secret returns error",
		wantErr: true,
	}, {
		name: "empty secret returns error",
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
		wantErr: true,
	}, {
		name: "secret with some valid data",
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Data: secretData,
		},
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ctx, _ = injection.Fake.SetupInformers(ctx, &rest.Config{})

			if tt.secret != nil {
				require.NoError(t, secretinformer.Get(ctx).Informer().GetIndexer().Add(tt.secret))
			}

			getter := reconciler.NewCertificateGetter(secretinformer.Get(ctx).Lister(), namespace, name)
			got, err := getter.GenerateTLSDialer(&net.Dialer{})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NotNil(t, got)
			}
		})
	}
}
