package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	ctrl "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/test"
)

func TestMessageRouter_HandleServiceMessage_Matching(t *testing.T) {
	svc := test.NewServiceMock()

	counter := atomic.NewInt32(0)

	svc.MessageHandler(MessageRouter{
		1: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			counter.Inc()
		}),
	})

	inboundMsg := ctrl.NewInboundMessage(ctrl.ActualProtocolVersion, 0, 1, uuid.New(), nil)
	require.False(t, svc.InvokeMessageHandler(context.TODO(), &inboundMsg))
	require.Equal(t, int32(1), counter.Load())
}

func TestMessageRouter_HandleServiceMessage_NotMatching(t *testing.T) {
	svc := test.NewServiceMock()

	counter := atomic.NewInt32(0)

	svc.MessageHandler(MessageRouter{
		1: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			counter.Inc()
		}),
	})

	inboundMsg := ctrl.NewInboundMessage(ctrl.ActualProtocolVersion, 0, 10, uuid.New(), nil)
	require.True(t, svc.InvokeMessageHandler(context.TODO(), &inboundMsg))
	require.Equal(t, int32(0), counter.Load())
}
