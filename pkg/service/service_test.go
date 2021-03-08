package service_test

import (
	"context"
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

	inboundMessage := ctrl.NewInboundMessage(ctrl.ActualProtocolVersion, 0, uint8(ctrl.AckOpCode), outboundMessage.UUID(), nil)

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
	inboundMessage := ctrl.NewInboundMessage(ctrl.ActualProtocolVersion, 0, uint8(10), msgUuid, []byte(test.SomeMockPayload))
	mockConnection.InboundCh <- &inboundMessage

	wg.Wait()

	outboundMessage := <-mockConnection.OutboundCh
	require.Equal(t, uint8(ctrl.AckOpCode), outboundMessage.OpCode())
	require.Equal(t, msgUuid, outboundMessage.UUID())
}
