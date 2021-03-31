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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	podinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/pod/fake"
	"knative.dev/pkg/injection"

	"knative.dev/control-protocol/pkg/reconciler"
)

func TestPodIpGetter_GetAllPodsIp(t *testing.T) {
	namespace := "abc"

	tests := []struct {
		name string
		pods []*corev1.Pod
		want []string
	}{{
		name: "no pods",
		want: []string{},
	}, {
		name: "get one pod ip",
		pods: []*corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-a",
				Namespace: namespace,
			},
			Status: corev1.PodStatus{
				PodIP: "10.0.0.1",
			},
		}},
		want: []string{"10.0.0.1"},
	}, {
		name: "get different pod ip",
		pods: []*corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-a",
				Namespace: namespace,
			},
			Status: corev1.PodStatus{
				PodIP: "10.0.0.1",
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-b",
				Namespace: namespace,
			},
			Status: corev1.PodStatus{
				PodIP: "10.0.0.2",
			},
		}},
		want: []string{"10.0.0.1", "10.0.0.2"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			ctx := context.TODO()
			ctx, _ = injection.Fake.SetupInformers(ctx, &rest.Config{})

			for _, p := range tt.pods {
				require.NoError(t, podinformer.Get(ctx).Informer().GetIndexer().Add(p))
			}

			ipGetter := reconciler.PodIpGetter{
				Lister: podinformer.Get(ctx).Lister(),
			}
			got, err := ipGetter.GetAllPodsIp(namespace, labels.Everything())
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
