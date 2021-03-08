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

package reconciler

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	control "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/certificates"
	"knative.dev/control-protocol/pkg/network"
	"knative.dev/control-protocol/pkg/service"
)

type mockMessage string

func (m mockMessage) MarshalBinary() (data []byte, err error) {
	return []byte(m), nil
}

func (m *mockMessage) UnmarshalBinary(data []byte) error {
	*m = mockMessage(data)
	return nil
}

func parseMockMessage(bytes []byte) (interface{}, error) {
	var msg mockMessage
	err := (&msg).UnmarshalBinary(bytes)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

func mockValueMerger(old interface{}, new interface{}) interface{} {
	oldMsg := old.(*mockMessage)
	newMsg := new.(*mockMessage)
	merged := mockMessage(string(*oldMsg) + string(*newMsg))
	return &merged
}

var serverConnectionPoolSetupTestCases = map[string]func(t *testing.T, ctx context.Context, opts ...ControlPlaneConnectionPoolOption) (*network.ControlServer, *ControlPlaneConnectionPool){
	"InsecureConnectionPool": setupInsecureServerAndConnectionPool,
	"TLSConnectionPool": func(t *testing.T, ctx context.Context, opts ...ControlPlaneConnectionPoolOption) (*network.ControlServer, *ControlPlaneConnectionPool) {
		serverCtx, serverCancelFn := context.WithCancel(ctx)

		caKP, err := certificates.CreateCACerts(ctx, 24*time.Hour)
		require.NoError(t, err)
		caCert, caPrivateKey, err := caKP.Parse()
		require.NoError(t, err)

		clientTLSDialerFactory := mustGenerateTLSClientConf(t, ctx, caPrivateKey, caCert)

		server, err := network.StartControlServer(serverCtx, mustGenerateTLSServerConf(t, ctx, caPrivateKey, caCert))
		require.NoError(t, err)
		t.Cleanup(func() {
			serverCancelFn()
			<-server.ClosedCh()
		})

		connectionPool := NewControlPlaneConnectionPool(clientTLSDialerFactory, opts...)
		t.Cleanup(func() {
			connectionPool.Close(ctx)
		})

		return server, connectionPool
	},
}

func TestReconcileConnections(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())

	for name, setupFn := range serverConnectionPoolSetupTestCases {
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

			runSendReceiveTest(t, server, conns[address])

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

func TestCachingWrapper(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())
	for name, setupFn := range serverConnectionPoolSetupTestCases {
		t.Run(name, func(t *testing.T) {
			dataPlane, connectionPool := setupFn(t, ctx, WithServiceWrapper(service.WithCachingService(ctx)))
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
				require.NoError(t, controlPlane.SendAndWaitForAck(1, mockMessage("Funky!")))
			}

			require.Equal(t, int32(1), messageReceivedCounter.Load())
		})
	}
}

func mustGenerateTLSServerConf(t *testing.T, ctx context.Context, caKey *rsa.PrivateKey, caCertificate *x509.Certificate) func() (*tls.Config, error) {
	return func() (*tls.Config, error) {
		dataPlaneKeyPair, err := certificates.CreateDataPlaneCert(ctx, caKey, caCertificate, 24*time.Hour)
		require.NoError(t, err)

		dataPlaneCert, err := tls.X509KeyPair(dataPlaneKeyPair.CertBytes(), dataPlaneKeyPair.PrivateKeyBytes())
		require.NoError(t, err)

		certPool := x509.NewCertPool()
		certPool.AddCert(caCertificate)
		return &tls.Config{
			Certificates: []tls.Certificate{dataPlaneCert},
			ClientCAs:    certPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ServerName:   caCertificate.DNSNames[0],
		}, nil
	}
}

type mockTLSDialerFactory tls.Config

func (m *mockTLSDialerFactory) GenerateTLSDialer(baseDialOptions *net.Dialer) (*tls.Dialer, error) {
	// Copy from base dial options
	dialOptions := *baseDialOptions

	return &tls.Dialer{
		NetDialer: &dialOptions,
		Config:    (*tls.Config)(m),
	}, nil
}

func mustGenerateTLSClientConf(t *testing.T, ctx context.Context, caKey *rsa.PrivateKey, caCertificate *x509.Certificate) TLSDialerFactory {
	controlPlaneKeyPair, err := certificates.CreateControlPlaneCert(ctx, caKey, caCertificate, 24*time.Hour)
	require.NoError(t, err)

	controlPlaneCert, err := tls.X509KeyPair(controlPlaneKeyPair.CertBytes(), controlPlaneKeyPair.PrivateKeyBytes())
	require.NoError(t, err)

	certPool := x509.NewCertPool()
	certPool.AddCert(caCertificate)
	return &mockTLSDialerFactory{
		Certificates: []tls.Certificate{controlPlaneCert},
		RootCAs:      certPool,
		ServerName:   certificates.FakeDnsName,
	}
}

func runSendReceiveTest(t *testing.T, sender control.Service, receiver control.Service) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	receiver.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	require.NoError(t, sender.SendAndWaitForAck(1, mockMessage("Funky!")))

	wg.Wait()
}

func setupInsecureServerAndConnectionPool(t *testing.T, ctx context.Context, opts ...ControlPlaneConnectionPoolOption) (*network.ControlServer, *ControlPlaneConnectionPool) {
	serverCtx, serverCancelFn := context.WithCancel(ctx)

	controlServer, err := network.StartInsecureControlServer(serverCtx, network.WithPort(0))
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn()
		<-controlServer.ClosedCh()
	})

	connectionPool := NewInsecureControlPlaneConnectionPool(opts...)
	t.Cleanup(func() {
		connectionPool.Close(ctx)
	})

	return controlServer, connectionPool
}
