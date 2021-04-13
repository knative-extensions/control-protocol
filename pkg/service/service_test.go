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

package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ctrl "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/service"
	"knative.dev/control-protocol/pkg/test"
)

func TestService_SendAndWaitForAck(t *testing.T) {
	mockConnection := test.NewConnectionMock()

	ctx, cancelFn := context.WithCancel(context.TODO())
	t.Cleanup(cancelFn)

	svc := service.NewService(ctx, mockConnection)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := svc.SendAndWaitForAck(10, test.SomeMockPayload)
		require.NoError(t, err)
	}()

	outboundMessage := <-mockConnection.OutboundCh

	require.Equal(t, uint8(10), outboundMessage.OpCode())
	require.Equal(t, uint32(len([]byte(test.SomeMockPayload))), outboundMessage.Length())

	inboundMessage := ctrl.NewMessage(outboundMessage.UUID(), uint8(ctrl.AckOpCode), nil)

	mockConnection.InboundCh <- &inboundMessage

	wg.Wait()
}

func TestService_SendAndWaitForAckWithError(t *testing.T) {
	mockConnection := test.NewConnectionMock()

	ctx, cancelFn := context.WithCancel(context.TODO())
	t.Cleanup(cancelFn)

	svc := service.NewService(ctx, mockConnection)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := svc.SendAndWaitForAck(10, test.SomeMockPayload)
		assert.Error(t, err, "Ack contained an error: Some wacky error")
	}()

	outboundMessage := <-mockConnection.OutboundCh

	require.Equal(t, uint8(10), outboundMessage.OpCode())
	require.Equal(t, uint32(len([]byte(test.SomeMockPayload))), outboundMessage.Length())

	inboundMessage := ctrl.NewMessage(outboundMessage.UUID(), uint8(ctrl.AckOpCode), []byte("Some wacky error"))

	mockConnection.InboundCh <- &inboundMessage

	wg.Wait()
}

func TestService_MessageHandler(t *testing.T) {
	mockConnection := test.NewConnectionMock()

	ctx, cancelFn := context.WithCancel(context.TODO())
	t.Cleanup(cancelFn)

	var wg sync.WaitGroup
	wg.Add(1)

	svc := service.NewService(ctx, mockConnection)

	svc.MessageHandler(ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
		defer wg.Done()
		assert.Equal(t, uint8(10), message.Headers().OpCode())
		assert.Equal(t, []byte(test.SomeMockPayload), message.Payload())

		message.Ack()
	}))

	msgUuid := uuid.New()
	inboundMessage := ctrl.NewMessage(msgUuid, uint8(10), []byte(test.SomeMockPayload))
	mockConnection.InboundCh <- &inboundMessage

	wg.Wait()

	outboundMessage := <-mockConnection.OutboundCh
	require.Equal(t, uint8(ctrl.AckOpCode), outboundMessage.OpCode())
	require.Equal(t, msgUuid, outboundMessage.UUID())
}

func TestService_MessageHandler_AckWithError(t *testing.T) {
	mockConnection := test.NewConnectionMock()

	ctx, cancelFn := context.WithCancel(context.TODO())
	t.Cleanup(cancelFn)

	var wg sync.WaitGroup
	wg.Add(1)

	svc := service.NewService(ctx, mockConnection)

	svc.MessageHandler(ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
		defer wg.Done()
		assert.Equal(t, uint8(10), message.Headers().OpCode())
		assert.Equal(t, []byte(test.SomeMockPayload), message.Payload())

		message.AckWithError(errors.New("some wacky error"))
	}))

	msgUuid := uuid.New()
	inboundMessage := ctrl.NewMessage(msgUuid, uint8(10), []byte(test.SomeMockPayload))
	mockConnection.InboundCh <- &inboundMessage

	wg.Wait()

	outboundMessage := <-mockConnection.OutboundCh
	require.Equal(t, uint8(ctrl.AckOpCode), outboundMessage.OpCode())
	require.Equal(t, msgUuid, outboundMessage.UUID())
	require.Equal(t, []byte("some wacky error"), outboundMessage.Payload())
}

func TestService_ErrorHandler(t *testing.T) {
	mockConnection := test.NewConnectionMock()

	ctx, cancelFn := context.WithCancel(context.TODO())
	t.Cleanup(cancelFn)

	var wg sync.WaitGroup
	wg.Add(1)

	svc := service.NewService(ctx, mockConnection)

	svc.ErrorHandler(ctrl.ErrorHandlerFunc(func(ctx context.Context, err error) {
		defer wg.Done()
		assert.Error(t, err, "my err")
	}))

	mockConnection.ErrorsCh <- errors.New("my err")

	wg.Wait()
}
