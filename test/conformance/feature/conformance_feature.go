/*
 * Copyright 2021 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package feature

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"

	"knative.dev/control-protocol/pkg/certificates"
	"knative.dev/control-protocol/test/conformance/resources/conformance_client"

	"knative.dev/control-protocol/test/conformance/resources/conformance_server"
)

func ConformanceFeature() *feature.Feature {
	return conformanceFeature("ConformanceFeature", false)
}

func TLSConformanceFeature() *feature.Feature {
	return conformanceFeature("TLSConformanceFeature", true)
}

func conformanceFeature(featureName string, tls bool) *feature.Feature {
	f := feature.NewFeatureNamed(featureName)

	client := "client"
	server := "server"

	port := 10000

	if tls {
		// We need to generate the certs and push them in two secrets,
		// one for the server and one for the client
		// This is the job the certificates controller usually does for us
		f.Setup("Generate client and server certs", func(ctx context.Context, t feature.T) {
			// Generate the keys
			caKP, err := certificates.CreateCACerts(ctx, 24*time.Hour)
			require.NoError(t, err)
			caCert, caPrivateKey, err := caKP.Parse()
			require.NoError(t, err)

			dataPlanePipelineKeyPair, err := certificates.CreateCert(ctx, caPrivateKey, caCert, 24*time.Hour, certificates.DataPlanePipelinePrefix+"myns", certificates.LegacyFakeDnsName)
			require.NoError(t, err)

			dataPlaneEdgeKeyPair, err := certificates.CreateCert(ctx, caPrivateKey, caCert, 24*time.Hour, certificates.DataPlaneEdgePrefix+"myns", certificates.LegacyFakeDnsName)
			require.NoError(t, err)

			// Create the secrets
			clientSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      client + "-keys",
					Namespace: environment.FromContext(ctx).Namespace(),
				},
				Data: map[string][]byte{
					certificates.SecretCertKey:   dataPlanePipelineKeyPair.CertBytes(),
					certificates.SecretPKKey:     dataPlanePipelineKeyPair.PrivateKeyBytes(),
					certificates.SecretCaCertKey: caKP.CertBytes(),
				},
			}
			serverSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      server + "-keys",
					Namespace: environment.FromContext(ctx).Namespace(),
				},
				Data: map[string][]byte{
					certificates.SecretCertKey:   dataPlaneEdgeKeyPair.CertBytes(),
					certificates.SecretPKKey:     dataPlaneEdgeKeyPair.PrivateKeyBytes(),
					certificates.SecretCaCertKey: caKP.CertBytes(),
				},
			}

			// Push the secrets to k8s
			_, err = kubeclient.Get(ctx).CoreV1().Secrets(environment.FromContext(ctx).Namespace()).Create(
				ctx,
				&clientSecret,
				metav1.CreateOptions{},
			)
			require.NoError(t, err)
			_, err = kubeclient.Get(ctx).CoreV1().Secrets(environment.FromContext(ctx).Namespace()).Create(
				ctx,
				&serverSecret,
				metav1.CreateOptions{},
			)
			require.NoError(t, err)
		})

	}

	f.Setup("Start server", conformance_server.StartPod(server, port, tls))

	f.Requirement("Start client", func(ctx context.Context, t feature.T) {
		pod, err := kubeclient.Get(ctx).CoreV1().Pods(environment.FromContext(ctx).Namespace()).Get(ctx, server, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotEmpty(t, pod.Status.PodIP)

		conformance_client.StartJob(client, fmt.Sprintf("%s:%d", pod.Status.PodIP, port), tls)(ctx, t)
	})

	f.Stable("Send and receive").Must("Job should succeed", func(ctx context.Context, t feature.T) {
		require.NoError(t, k8s.WaitUntilJobDone(
			ctx,
			t,
			client,
		))
	}).Must("Pod shouldn't be failed", func(ctx context.Context, t feature.T) {
		pod, err := kubeclient.Get(ctx).CoreV1().Pods(environment.FromContext(ctx).Namespace()).Get(ctx, server, metav1.GetOptions{})
		require.NoError(t, err)
		require.Contains(t, []corev1.PodPhase{corev1.PodRunning, corev1.PodSucceeded}, pod.Status.Phase)
	})

	return f
}
