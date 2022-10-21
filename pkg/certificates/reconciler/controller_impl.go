/*
Copyright 2022 The Knative Authors

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
	context "context"
	fmt "fmt"
	reflect "reflect"
	strings "strings"

	zap "go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	watch "k8s.io/apimachinery/pkg/watch"
	informersv1 "k8s.io/client-go/informers/core/v1"
	scheme "k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	record "k8s.io/client-go/tools/record"
	client "knative.dev/pkg/client/injection/kube/client"
	secretreconciler "knative.dev/pkg/client/injection/kube/reconciler/core/v1/secret"
	controller "knative.dev/pkg/controller"
	logging "knative.dev/pkg/logging"
	logkey "knative.dev/pkg/logging/logkey"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	defaultControllerAgentName = "secret-controller"
	defaultFinalizerName       = "secrets.core"
)

// Interface defines the strongly typed interfaces to be implemented by a
// controller reconciling v1.Secret.
type Interface interface {
	// ReconcileKind implements custom logic to reconcile v1.Secret. Any changes
	// to the objects .Status or .Finalizers will be propagated to the stored
	// object. It is recommended that implementors do not call any update calls
	// for the Kind inside of ReconcileKind, it is the responsibility of the calling
	// controller to propagate those properties. The resource passed to ReconcileKind
	// will always have an empty deletion timestamp.
	ReconcileKind(ctx context.Context, o *corev1.Secret) pkgreconciler.Event
}

// NewFilteredImpl returns a controller.Impl that handles queuing and feeding work from
// the queue through an implementation of controller.Reconciler, delegating to
// the provided Interface and optional Finalizer methods. OptionsFn is used to return
// controller.ControllerOptions to be used by the internal reconciler.
func NewFilteredImpl(ctx context.Context, r Interface, secretInformer informersv1.SecretInformer, options ...controller.Options) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Check the options function input. It should be 0 or 1.
	if len(options) > 1 {
		logger.Fatal("Up to one options function is supported, found: ", len(options))
	}

	lister := secretInformer.Lister()

	agentName := defaultControllerAgentName
	recorder := controller.GetEventRecorder(ctx)
	if recorder == nil {
		// Create event broadcaster
		logger.Debug("Creating event broadcaster")
		eventBroadcaster := record.NewBroadcaster()
		watches := []watch.Interface{
			eventBroadcaster.StartLogging(logger.Named("event-broadcaster").Infof),
			eventBroadcaster.StartRecordingToSink(
				&v1.EventSinkImpl{Interface: client.Get(ctx).CoreV1().Events("")}),
		}
		recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: agentName})
		go func() {
			<-ctx.Done()
			for _, w := range watches {
				w.Stop()
			}
		}()
	}

	rec := secretreconciler.NewReconciler(ctx, logger, client.Get(ctx), lister, recorder, r, options...)

	ctrType := reflect.TypeOf(r).Elem()
	ctrTypeName := fmt.Sprintf("%s.%s", ctrType.PkgPath(), ctrType.Name())
	ctrTypeName = strings.ReplaceAll(ctrTypeName, "/", ".")

	logger = logger.With(
		zap.String(logkey.ControllerType, ctrTypeName),
		zap.String(logkey.Kind, "core.Secret"),
	)

	impl := controller.NewContext(ctx, rec, controller.ControllerOptions{WorkQueueName: ctrTypeName, Logger: logger})

	return impl
}

func init() {
	scheme.AddToScheme(scheme.Scheme)
}
