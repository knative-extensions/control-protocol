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
	"log"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/reconciler-test/pkg/environment"
	"knative.dev/reconciler-test/pkg/feature"
	"knative.dev/reconciler-test/pkg/k8s"

	"knative.dev/control-protocol/pkg/certificates"
	"knative.dev/control-protocol/test/conformance/resources/conformance_client"

	"knative.dev/control-protocol/test/conformance/resources/conformance_server"
)

func ConformanceFeature(clientImage string, serverImage string) *feature.Feature {
	return conformanceFeature("ConformanceFeature", clientImage, serverImage, false)
}

func TLSConformanceFeature(clientImage string, serverImage string) *feature.Feature {
	return conformanceFeature("TLSConformanceFeature", clientImage, serverImage, true)
}

func conformanceFeature(featureName string, clientImage string, serverImage string, tls bool) *feature.Feature {
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

			controlPlaneKeyPair, err := certificates.CreateControlPlaneCert(ctx, caPrivateKey, caCert, 24*time.Hour)
			require.NoError(t, err)

			dataPlaneKeyPair, err := certificates.CreateDataPlaneCert(ctx, caPrivateKey, caCert, 24*time.Hour)
			require.NoError(t, err)

			// Create the secrets
			clientSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      client + "-keys",
					Namespace: environment.FromContext(ctx).Namespace(),
				},
				Data: map[string][]byte{
					certificates.SecretCertKey:   controlPlaneKeyPair.CertBytes(),
					certificates.SecretPKKey:     controlPlaneKeyPair.PrivateKeyBytes(),
					certificates.SecretCaCertKey: caKP.CertBytes(),
				},
			}
			serverSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      server + "-keys",
					Namespace: environment.FromContext(ctx).Namespace(),
				},
				Data: map[string][]byte{
					certificates.SecretCertKey:   dataPlaneKeyPair.CertBytes(),
					certificates.SecretPKKey:     dataPlaneKeyPair.PrivateKeyBytes(),
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

	f.Setup("Start server", conformance_server.StartPod(server, serverImage, port, tls))
	f.Setup("Wait for server ready", func(ctx context.Context, t feature.T) {
		// TODO - DEBUG ONLY - DO NOT MERGE !!!
		//k8s.WaitForPodRunningOrFail(ctx, t, server)
		customWaitForPodRunningOrFail(ctx, t, server)
	})
	f.Setup("Start client", func(ctx context.Context, t feature.T) {
		pod, err := kubeclient.Get(ctx).CoreV1().Pods(environment.FromContext(ctx).Namespace()).Get(ctx, server, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotEmpty(t, pod.Status.PodIP)

		conformance_client.StartJob(client, clientImage, fmt.Sprintf("%s:%d", pod.Status.PodIP, port), tls)(ctx, t)
	})

	f.Stable("Send and receive").Must("Job should succeed", func(ctx context.Context, t feature.T) {
		require.NoError(t, k8s.WaitUntilJobDone(
			ctx,
			kubeclient.Get(ctx),
			environment.FromContext(ctx).Namespace(),
			client,
		))
	}).Must("Pod shouldn't be failed", func(ctx context.Context, t feature.T) {
		pod, err := kubeclient.Get(ctx).CoreV1().Pods(environment.FromContext(ctx).Namespace()).Get(ctx, server, metav1.GetOptions{})
		require.NoError(t, err)
		require.Contains(t, []corev1.PodPhase{corev1.PodRunning, corev1.PodSucceeded}, pod.Status.Phase)
	})

	return f
}

// TODO - DEBUG ONLY - DO NOT MERGE !!!
func customWaitForPodRunningOrFail(ctx context.Context, t feature.T, podName string) {
	log.Println(fmt.Sprintf("podName = %s", podName))
	log.Println(fmt.Sprintf("namespace = %s", environment.FromContext(ctx).Namespace()))
	podClient := kubeclient.Get(ctx).CoreV1().Pods(environment.FromContext(ctx).Namespace())
	p := podClient
	interval, timeout := k8s.PollTimings(ctx, nil)
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		p, err := p.Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			log.Println(fmt.Sprintf("*** DEBUG 1 *** p.Get() error = %+v", err))
			return true, err
		}
		return customPodRunning(p), nil // TODO - Expected To Time-Out Here?
	})
	if err != nil {
		log.Println(fmt.Sprintf("*** DEBUG 2 *** wait.PollImmediate() error = %+v", err))
		sb := strings.Builder{}
		if p, err := podClient.Get(ctx, podName, metav1.GetOptions{}); err != nil {
			log.Println(fmt.Sprintf("*** DEBUG 3 *** podClient.Get() error = %+v", err))
			sb.WriteString(err.Error())
			sb.WriteString("\n")
		} else {
			for _, c := range p.Spec.Containers {
				if b, err := k8s.PodLogs(ctx, podName, c.Name, environment.FromContext(ctx).Namespace()); err != nil {
					log.Println(fmt.Sprintf("*** DEBUG 4 *** k8s.PodLogs() error = %+v", err))
					sb.WriteString(err.Error())
				} else {
					sb.Write(b)
				}
				sb.WriteString("\n")
			}
		}
		log.Println(fmt.Sprintf("*** DEBUG 5 *** sb = %s", sb.String()))
		t.Fatalf("Failed while waiting for pod %s running: %+v", podName, errors.WithStack(err)) // TODO - Log "sb" here?
	} else {
		log.Println("*** DEBUG 6 *** Pod Detected As Running Or Succeeded!")
	}
}

// TODO - DEBUG ONLY - DO NOT MERGE !!!
func customPodRunning(pod *corev1.Pod) bool {
	log.Println(fmt.Sprintf("*** DEBUG A *** pod.Status.Phase = %s", pod.Status.Phase))
	return pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodSucceeded
}
