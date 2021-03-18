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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	control "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/reconciler"
	"knative.dev/control-protocol/pkg/service"
	"knative.dev/control-protocol/pkg/test"
)

func TestReconcileConnections(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())

	for name, setupFn := range test.ConnectionPoolTestCases() {
		t.Run(name, func(t *testing.T) {
			server, connectionPool := setupFn(t, ctx)
			address := fmt.Sprintf("127.0.0.1:%d", server.ListeningPort())

			newServiceInvokedCounter := atomic.NewInt32(0)
			oldServiceInvokedCounter := atomic.NewInt32(0)

			conns, err := connectionPool.ReconcileConnections(context.TODO(), "hello", []string{address}, func(string, control.Service) {
				newServiceInvokedCounter.Inc()
			}, func(string) {
				oldServiceInvokedCounter.Inc()
			})
			require.NoError(t, err)
			require.Contains(t, conns, address)
			require.Equal(t, int32(1), newServiceInvokedCounter.Load())
			require.Equal(t, int32(0), oldServiceInvokedCounter.Load())

			test.SendReceiveTest(t, server, conns[address])

			newServiceInvokedCounter.Store(0)
			oldServiceInvokedCounter.Store(0)

			conns, err = connectionPool.ReconcileConnections(context.TODO(), "hello", []string{}, func(string, control.Service) {
				newServiceInvokedCounter.Inc()
			}, func(string) {
				oldServiceInvokedCounter.Inc()
			})
			require.NoError(t, err)
			require.NotContains(t, conns, address)
			require.Equal(t, int32(0), newServiceInvokedCounter.Load())
			require.Equal(t, int32(1), oldServiceInvokedCounter.Load())
		})
	}
}

func TestReconcileConnections_ResolveAndRemove(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())

	for name, setupFn := range test.ConnectionPoolTestCases() {
		t.Run(name, func(t *testing.T) {
			server, connectionPool := setupFn(t, ctx)
			address := fmt.Sprintf("127.0.0.1:%d", server.ListeningPort())

			conns, err := connectionPool.ReconcileConnections(context.TODO(), "hello", []string{address}, nil, nil)
			require.NoError(t, err)
			require.Contains(t, conns, address)

			_, resolvedConn := connectionPool.ResolveControlInterface("hello", address)
			require.Same(t, conns[address], resolvedConn)

			connectionPool.RemoveAllConnections(ctx, "hello")

			_, resolvedConn = connectionPool.ResolveControlInterface("hello", address)
			require.Nil(t, resolvedConn)
		})
	}
}

func TestConnectionPool_IntegrationWithCachingWrapper(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())

	for name, setupFn := range test.ConnectionPoolTestCases() {
		t.Run(name, func(t *testing.T) {
			dataPlane, connectionPool := setupFn(t, ctx, reconciler.WithServiceWrapper(service.WithCachingService(ctx)))
			address := fmt.Sprintf("127.0.0.1:%d", dataPlane.ListeningPort())

			conns, err := connectionPool.ReconcileConnections(context.TODO(), "hello", []string{address}, nil, nil)
			require.NoError(t, err)
			require.Contains(t, conns, address)

			controlPlane := conns[address]

			messageReceivedCounter := atomic.NewInt32(0)

			dataPlane.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
				require.Equal(t, uint8(1), message.Headers().OpCode())
				require.Equal(t, "Funky!", string(message.Payload()))
				message.Ack()
				messageReceivedCounter.Inc()
			}))

			for i := 0; i < 10; i++ {
				require.NoError(t, controlPlane.SendAndWaitForAck(1, test.MockPayload("Funky!")))
			}

			require.Equal(t, int32(1), messageReceivedCounter.Load())
		})
	}
}
