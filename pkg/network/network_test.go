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

package network_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	control "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/network"
	"knative.dev/control-protocol/pkg/test"
)

func TestTLSConf(t *testing.T) {
	serverTLSConf, clientTLSDialer := test.MustGenerateTestTLSConf(t, context.TODO())
	require.NotNil(t, serverTLSConf)
	require.NotNil(t, clientTLSDialer)
}

func TestNoopMessageHandlerAcks(t *testing.T) {
	_, server, _, _ := test.MustSetupSecureControlPair(t)
	require.NoError(t, server.SendAndWaitForAck(10, test.SomeMockPayload))
}

func TestStartClientAndServer(t *testing.T) {
	_, _, _, _ = test.MustSetupSecureControlPair(t)
}

func TestServerToClient(t *testing.T) {
	_, server, _, client := test.MustSetupSecureControlPair(t)
	test.SendReceiveTest(t, server, client)
}

func TestClientToServer(t *testing.T) {
	_, server, _, client := test.MustSetupSecureControlPair(t)
	test.SendReceiveTest(t, client, server)
}

func TestInsecureServerToClient(t *testing.T) {
	_, server, _, client := test.MustSetupInsecureControlPair(t)
	test.SendReceiveTest(t, server, client)
}

func TestInsecureClientToServer(t *testing.T) {
	_, server, _, client := test.MustSetupInsecureControlPair(t)
	test.SendReceiveTest(t, client, server)
}

func TestServerToClientAndBack(t *testing.T) {
	_, server, _, client := test.MustSetupSecureControlPair(t)

	wg := sync.WaitGroup{}
	wg.Add(6)

	server.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(2), message.Headers().OpCode())
		require.Equal(t, "Funky2!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))
	client.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	require.NoError(t, server.SendAndWaitForAck(1, test.MockPayload("Funky!")))
	require.NoError(t, client.SendAndWaitForAck(2, test.MockPayload("Funky2!")))
	require.NoError(t, server.SendAndWaitForAck(1, test.MockPayload("Funky!")))
	require.NoError(t, client.SendAndWaitForAck(2, test.MockPayload("Funky2!")))
	require.NoError(t, server.SendAndWaitForAck(1, test.MockPayload("Funky!")))
	require.NoError(t, client.SendAndWaitForAck(2, test.MockPayload("Funky2!")))

	wg.Wait()
}

func TestServerToClientAndBackWithErrorAck(t *testing.T) {
	_, server, _, client := test.MustSetupSecureControlPair(t)

	wg := sync.WaitGroup{}
	wg.Add(1)

	server.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		assert.Equal(t, uint8(2), message.Headers().OpCode())
		assert.Equal(t, "Funky!", string(message.Payload()))
		message.AckWithError(errors.New("abc"))
		wg.Done()
	}))

	require.Error(t, client.SendAndWaitForAck(2, test.MockPayload("Funky!")), "abc")

	wg.Wait()
}

func TestClientToServerWithClientDisconnection(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())
	serverTLSConf, clientTLSDialer := test.MustGenerateTestTLSConf(t, ctx)

	clientCtx, clientCancelFn := context.WithCancel(ctx)
	serverCtx, serverCancelFn := context.WithCancel(ctx)
	t.Cleanup(clientCancelFn)
	t.Cleanup(serverCancelFn)

	controlServer, err := network.StartControlServer(serverCtx, serverTLSConf, network.WithPort(0))
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn()
		<-controlServer.ClosedCh()
	})

	client, err := network.StartControlClient(clientCtx, clientTLSDialer, fmt.Sprintf("localhost:%d", controlServer.ListeningPort()))
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(2)

	controlServer.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	// Send a message, close the client, restart it and send another message

	require.NoError(t, client.SendAndWaitForAck(1, test.MockPayload("Funky!")))

	clientCancelFn()

	time.Sleep(1 * time.Second)

	clientCtx2, clientCancelFn2 := context.WithCancel(ctx)
	t.Cleanup(clientCancelFn2)
	client2, err := network.StartControlClient(clientCtx2, clientTLSDialer, fmt.Sprintf("localhost:%d", controlServer.ListeningPort()))
	require.NoError(t, err)

	require.NoError(t, client2.SendAndWaitForAck(1, test.MockPayload("Funky!")))

	wg.Wait()
}

func TestClientToServerWithServerDisconnection(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())
	serverTLSConf, clientTLSDialer := test.MustGenerateTestTLSConf(t, ctx)

	clientCtx, clientCancelFn := context.WithCancel(ctx)
	serverCtx, serverCancelFn := context.WithCancel(ctx)
	t.Cleanup(clientCancelFn)
	t.Cleanup(serverCancelFn)

	controlServer, err := network.StartControlServer(serverCtx, serverTLSConf)
	require.NoError(t, err)

	client, err := network.StartControlClient(clientCtx, clientTLSDialer, fmt.Sprintf("127.0.0.1:%d", controlServer.ListeningPort()))
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	controlServer.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	// Send a message, close the server, restart it and send another message

	require.NoError(t, client.SendAndWaitForAck(1, test.MockPayload("Funky!")))

	serverCancelFn()

	<-controlServer.ClosedCh()

	serverCtx2, serverCancelFn2 := context.WithCancel(ctx)
	controlServer2, err := network.StartControlServer(serverCtx2, serverTLSConf, network.WithPort(controlServer.ListeningPort()))
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn2()
		<-controlServer2.ClosedCh()
	})

	controlServer2.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	require.NoError(t, client.SendAndWaitForAck(1, test.MockPayload("Funky!")))

	wg.Wait()
}

func TestManyMessages(t *testing.T) {
	ctx, server, _, client := test.MustSetupSecureControlPair(t)

	var wg sync.WaitGroup
	wg.Add(1000 * 2)

	processed := atomic.NewInt32(0)

	server.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(2), message.Headers().OpCode())
		require.Equal(t, "Funky2!", string(message.Payload()))
		message.Ack()
		wg.Done()
		processed.Inc()
	}))
	client.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
		processed.Inc()
	}))

	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			go func() {
				require.NoError(t, server.SendAndWaitForAck(1, test.MockPayload("Funky!")))
				wg.Done()
			}()
		} else {
			go func() {
				require.NoError(t, client.SendAndWaitForAck(2, test.MockPayload("Funky2!")))
				wg.Done()
			}()
		}
	}

	wg.Wait()

	logging.FromContext(ctx).Infof("Processed: %d", processed.Load())
}
