package service

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	control "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/test"
)

func TestCachingService(t *testing.T) {
	mockConnection := test.NewConnectionMock()

	ctx, cancelFn := context.WithCancel(context.TODO())
	t.Cleanup(cancelFn)

	svc := WithCachingService(ctx)(NewService(ctx, mockConnection))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			assert.NoError(t, svc.SendAndWaitForAck(10, test.SomeMockPayload))
		}
	}()

	outboundMessage := <-mockConnection.OutboundCh
	require.Equal(t, uint8(10), outboundMessage.OpCode())
	require.Equal(t, uint32(len([]byte(test.SomeMockPayload))), outboundMessage.Length())

	inboundMessage := control.NewInboundMessage(control.ActualProtocolVersion, 0, uint8(control.AckOpCode), outboundMessage.UUID(), nil)
	mockConnection.InboundCh <- &inboundMessage

	wg.Wait()

	require.Empty(t, mockConnection.ConsumeOutboundMessages()) // No more outbound messages, just one
}
