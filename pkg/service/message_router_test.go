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
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	ctrl "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/service"
	"knative.dev/control-protocol/pkg/test"
)

func TestMessageRouter_HandleServiceMessage_Matching(t *testing.T) {
	svc := test.NewServiceMock()

	counter := atomic.NewInt32(0)

	svc.MessageHandler(service.MessageRouter{
		1: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			counter.Inc()
		}),
	})

	inboundMsg := ctrl.NewMessage(uuid.New(), 1, nil, ctrl.WithFlags(0), ctrl.WithVersion(ctrl.ActualProtocolVersion))
	require.False(t, svc.InvokeMessageHandler(context.TODO(), &inboundMsg))
	require.Equal(t, int32(1), counter.Load())
}

func TestMessageRouter_HandleServiceMessage_NotMatching(t *testing.T) {
	svc := test.NewServiceMock()

	counter := atomic.NewInt32(0)

	svc.MessageHandler(service.MessageRouter{
		1: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			counter.Inc()
		}),
	})

	inboundMsg := ctrl.NewMessage(uuid.New(), 10, nil, ctrl.WithFlags(0), ctrl.WithVersion(ctrl.ActualProtocolVersion))
	require.True(t, svc.InvokeMessageHandler(context.TODO(), &inboundMsg))
	require.Equal(t, int32(0), counter.Load())
}

func TestMessageRouterIntegration(t *testing.T) {
	_, dataPlane, _, controlPlane := test.MustSetupInsecureControlPair(t)

	opcode1Count := atomic.NewInt32(0)
	opcode2Count := atomic.NewInt32(0)

	dataPlane.MessageHandler(service.MessageRouter{
		1: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			require.Equal(t, uint8(1), message.Headers().OpCode())
			require.Equal(t, "Funky!", string(message.Payload()))
			message.Ack()
			opcode1Count.Inc()
		}),
		2: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			require.Equal(t, uint8(2), message.Headers().OpCode())
			require.Equal(t, "Funky!", string(message.Payload()))
			message.Ack()
			opcode2Count.Inc()
		}),
	})

	for i := 0; i < 10; i++ {
		require.NoError(t, controlPlane.SendAndWaitForAck(ctrl.OpCode((i%2)+1), test.MockPayload("Funky!")))
	}

	require.Equal(t, int32(5), opcode1Count.Load())
	require.Equal(t, int32(5), opcode2Count.Load())
}

func TestMessageRouterIntegration_MessageNotMatchingAck(t *testing.T) {
	_, dataPlane, _, controlPlane := test.MustSetupInsecureControlPair(t)

	opcode1Count := atomic.NewInt32(0)
	opcode2Count := atomic.NewInt32(0)

	dataPlane.MessageHandler(service.MessageRouter{
		1: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			require.Equal(t, uint8(1), message.Headers().OpCode())
			require.Equal(t, "Funky!", string(message.Payload()))
			message.Ack()
			opcode1Count.Inc()
		}),
		2: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			require.Equal(t, uint8(2), message.Headers().OpCode())
			require.Equal(t, "Funky!", string(message.Payload()))
			message.Ack()
			opcode2Count.Inc()
		}),
	})

	require.NotNil(t, controlPlane.SendAndWaitForAck(10, test.MockPayload("Funky!")))

	require.Equal(t, int32(0), opcode1Count.Load())
	require.Equal(t, int32(0), opcode2Count.Load())
}
