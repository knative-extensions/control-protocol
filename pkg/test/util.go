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

package test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	control "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/network"
	"knative.dev/control-protocol/pkg/reconciler"
)

// MustSetupSecureControlPair setup a server and a client using tls
func MustSetupSecureControlPair(t *testing.T) (context.Context, control.Service, context.Context, control.Service) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())
	serverTLSConf, clientTLSDialer := MustGenerateTestTLSConf(t, ctx)

	clientCtx, clientCancelFn := context.WithCancel(ctx)
	serverCtx, serverCancelFn := context.WithCancel(ctx)

	controlServer, err := network.StartControlServer(serverCtx, serverTLSConf, network.WithPort(0))
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn()
		<-controlServer.ClosedCh()
	})

	client, err := network.StartControlClient(clientCtx, clientTLSDialer, fmt.Sprintf("127.0.0.1:%d", controlServer.ListeningPort()))
	require.NoError(t, err)
	t.Cleanup(clientCancelFn)

	return serverCtx, controlServer, clientCtx, client
}

// MustSetupInsecureControlPair setup a plain connection with a server and a client
func MustSetupInsecureControlPair(t *testing.T) (context.Context, control.Service, context.Context, control.Service) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())

	clientCtx, clientCancelFn := context.WithCancel(ctx)
	serverCtx, serverCancelFn := context.WithCancel(ctx)

	controlServer, err := network.StartInsecureControlServer(serverCtx, network.WithPort(0))
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn()
		<-controlServer.ClosedCh()
	})

	client, err := network.StartControlClient(clientCtx, &net.Dialer{
		KeepAlive: network.KeepAlive,
		Deadline:  time.Time{},
	}, fmt.Sprintf("127.0.0.1:%d", controlServer.ListeningPort()))
	require.NoError(t, err)
	t.Cleanup(clientCancelFn)

	return serverCtx, controlServer, clientCtx, client
}

type MockTLSDialerFactory tls.Dialer

func (m *MockTLSDialerFactory) GenerateTLSDialer(*net.Dialer) (*tls.Dialer, error) {
	return (*tls.Dialer)(m), nil
}

func MustSetupInsecureControlWithPool(t *testing.T, ctx context.Context, opts ...reconciler.ControlPlaneConnectionPoolOption) (*network.ControlServer, reconciler.ControlPlaneConnectionPool) {
	serverCtx, serverCancelFn := context.WithCancel(ctx)

	controlServer, err := network.StartInsecureControlServer(serverCtx, network.WithPort(0))
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn()
		<-controlServer.ClosedCh()
	})

	connectionPool := reconciler.NewInsecureControlPlaneConnectionPool(opts...)
	t.Cleanup(func() {
		connectionPool.Close(ctx)
	})

	return controlServer, connectionPool
}

func MustSetupSecureControlWithPool(t *testing.T, ctx context.Context, opts ...reconciler.ControlPlaneConnectionPoolOption) (*network.ControlServer, reconciler.ControlPlaneConnectionPool) {
	serverCtx, serverCancelFn := context.WithCancel(ctx)

	serverTlsConf, clientDialer := MustGenerateTestTLSConf(t, ctx)

	server, err := network.StartControlServer(serverCtx, serverTlsConf, network.WithPort(0))
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn()
		<-server.ClosedCh()
	})

	connectionPool := reconciler.NewControlPlaneConnectionPool((*MockTLSDialerFactory)(clientDialer), opts...)
	t.Cleanup(func() {
		connectionPool.Close(ctx)
	})

	return server, connectionPool
}

func SendReceiveTest(t *testing.T, sender control.Service, receiver control.Service) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	receiver.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	require.NoError(t, sender.SendAndWaitForAck(1, MockPayload("Funky!")))

	wg.Wait()
}

func ConnectionPoolTestCases() map[string]func(t *testing.T, ctx context.Context, opts ...reconciler.ControlPlaneConnectionPoolOption) (*network.ControlServer, reconciler.ControlPlaneConnectionPool) {
	return map[string]func(t *testing.T, ctx context.Context, opts ...reconciler.ControlPlaneConnectionPoolOption) (*network.ControlServer, reconciler.ControlPlaneConnectionPool){
		"InsecureConnectionPool": MustSetupInsecureControlWithPool,
		"TLSConnectionPool":      MustSetupSecureControlWithPool,
	}
}
