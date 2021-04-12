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
	"encoding"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ctrl "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/service"
	"knative.dev/control-protocol/pkg/test"
)

type sentMessagesSvcMock map[ctrl.OpCode]interface{}

func (s sentMessagesSvcMock) SendAndWaitForAck(opcode ctrl.OpCode, payload encoding.BinaryMarshaler) error {
	s[opcode] = payload
	return nil
}

func (s sentMessagesSvcMock) MessageHandler(ctrl.MessageHandler) {
	panic("this shouldn't be invoked")
}

func (s sentMessagesSvcMock) ErrorHandler(ctrl.ErrorHandler) {
	panic("this shouldn't be invoked")
}

func TestNewAsyncCommandHandler_Success(t *testing.T) {
	sendingSvc := sentMessagesSvcMock{}
	receivingSvc := test.NewServiceMock()

	receivingSvc.MessageHandler(service.NewAsyncCommandHandler(func(ctx context.Context, message ctrl.ServiceMessage) ([]byte, error) {
		message.Ack()
		return service.Int64CommandId(1), nil
	}, sendingSvc, ctrl.OpCode(2)))

	msg := ctrl.NewMessage(uuid.New(), 1, []byte{0, 1, 2, 3})
	assert.True(t, receivingSvc.InvokeMessageHandler(
		context.TODO(),
		&msg,
	))

	require.Contains(t, sendingSvc, ctrl.OpCode(2))
	require.Equal(t, service.AsyncCommandResult{
		CommandId: service.Int64CommandId(1),
		Error:     "",
	}, sendingSvc[ctrl.OpCode(2)])
}

func TestNewAsyncCommandHandler_Error(t *testing.T) {
	sendingSvc := sentMessagesSvcMock{}
	receivingSvc := test.NewServiceMock()

	receivingSvc.MessageHandler(service.NewAsyncCommandHandler(func(ctx context.Context, message ctrl.ServiceMessage) ([]byte, error) {
		message.Ack()
		return service.Int64CommandId(1), errors.New("failure")
	}, sendingSvc, ctrl.OpCode(2)))

	msg := ctrl.NewMessage(uuid.New(), 1, []byte{0, 1, 2, 3})
	assert.True(t, receivingSvc.InvokeMessageHandler(
		context.TODO(),
		&msg,
	))

	require.Contains(t, sendingSvc, ctrl.OpCode(2))
	require.Equal(t, service.AsyncCommandResult{
		CommandId: service.Int64CommandId(1),
		Error:     "failure",
	}, sendingSvc[ctrl.OpCode(2)])
}

func TestAsyncCommandResult_RoundTrip(t *testing.T) {
	testCases := map[string]service.AsyncCommandResult{
		"positive generation": {
			CommandId: service.Int64CommandId(1),
		},
		"negative generation": {
			CommandId: service.Int64CommandId(-1),
		},
		"with error": {
			CommandId: service.Int64CommandId(1),
			Error:     "funky error",
		},
	}

	for testName, commandResult := range testCases {
		t.Run(testName, func(t *testing.T) {
			marshalled, err := commandResult.MarshalBinary()
			require.NoError(t, err)

			expectedLength := 4 + len(commandResult.CommandId)
			if commandResult.IsFailed() {
				expectedLength = expectedLength + 4 + len(commandResult.Error)
			}
			require.Len(t, marshalled, expectedLength)

			have := service.AsyncCommandResult{}
			require.NoError(t, have.UnmarshalBinary(marshalled))

			require.Equal(t, commandResult, have)
		})
	}
}
