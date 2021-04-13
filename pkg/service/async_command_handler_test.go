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
	"encoding/binary"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ctrl "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/message"
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

type mockAsyncCommand struct {
	generation int64
}

func (m *mockAsyncCommand) MarshalBinary() (data []byte, err error) {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(m.generation))
	return b, nil
}

func (m *mockAsyncCommand) UnmarshalBinary(data []byte) error {
	m.generation = int64(binary.BigEndian.Uint64(data))
	return nil
}

func (m *mockAsyncCommand) SerializedId() []byte {
	return message.Int64CommandId(m.generation)
}

func TestNewAsyncCommandHandler_Success(t *testing.T) {
	sendingSvc := sentMessagesSvcMock{}
	receivingSvc := test.NewServiceMock()

	receivingSvc.MessageHandler(service.NewAsyncCommandHandler(
		sendingSvc,
		&mockAsyncCommand{},
		ctrl.OpCode(2),
		func(ctx context.Context, commandMessage service.AsyncCommandMessage) {
			assert.Equal(t, uint8(1), commandMessage.Headers().OpCode())
			commandMessage.NotifySuccess()
		},
	))

	msg := ctrl.NewMessage(uuid.New(), 1, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	assert.True(t, receivingSvc.InvokeMessageHandler(
		context.TODO(),
		&msg,
	))

	require.Contains(t, sendingSvc, ctrl.OpCode(2))
	require.Equal(t, message.AsyncCommandResult{
		CommandId: message.Int64CommandId(1),
		Error:     "",
	}, sendingSvc[ctrl.OpCode(2)])
}

func TestNewAsyncCommandHandler_Failed_Nil(t *testing.T) {
	sendingSvc := sentMessagesSvcMock{}
	receivingSvc := test.NewServiceMock()

	receivingSvc.MessageHandler(service.NewAsyncCommandHandler(
		sendingSvc,
		&mockAsyncCommand{},
		ctrl.OpCode(2),
		func(ctx context.Context, commandMessage service.AsyncCommandMessage) {
			commandMessage.NotifyFailed(nil)
		},
	))

	msg := ctrl.NewMessage(uuid.New(), 1, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	assert.True(t, receivingSvc.InvokeMessageHandler(
		context.TODO(),
		&msg,
	))

	require.Contains(t, sendingSvc, ctrl.OpCode(2))
	require.Equal(t, message.AsyncCommandResult{
		CommandId: message.Int64CommandId(1),
		Error:     "",
	}, sendingSvc[ctrl.OpCode(2)])
}

func TestNewAsyncCommandHandler_Failed(t *testing.T) {
	sendingSvc := sentMessagesSvcMock{}
	receivingSvc := test.NewServiceMock()

	receivingSvc.MessageHandler(service.NewAsyncCommandHandler(
		sendingSvc,
		&mockAsyncCommand{},
		ctrl.OpCode(2),
		func(ctx context.Context, commandMessage service.AsyncCommandMessage) {
			commandMessage.NotifyFailed(errors.New("failure"))
		},
	))

	msg := ctrl.NewMessage(uuid.New(), 1, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	assert.True(t, receivingSvc.InvokeMessageHandler(
		context.TODO(),
		&msg,
	))

	require.Contains(t, sendingSvc, ctrl.OpCode(2))
	require.Equal(t, message.AsyncCommandResult{
		CommandId: message.Int64CommandId(1),
		Error:     "failure",
	}, sendingSvc[ctrl.OpCode(2)])
}

type mockAsyncCommandWithUnmarshalFailure struct{}

func (m *mockAsyncCommandWithUnmarshalFailure) MarshalBinary() (data []byte, err error) {
	panic("this shouldn't be called")
}

func (m *mockAsyncCommandWithUnmarshalFailure) UnmarshalBinary(data []byte) error {
	return errors.New("unmarshal error")
}

func (m *mockAsyncCommandWithUnmarshalFailure) SerializedId() []byte {
	panic("this shouldn't be called")
}

func TestNewAsyncCommandHandler_BadMessage(t *testing.T) {
	sendingSvc := sentMessagesSvcMock{}
	receivingSvc := test.NewServiceMock()

	receivingSvc.MessageHandler(service.NewAsyncCommandHandler(
		sendingSvc,
		&mockAsyncCommandWithUnmarshalFailure{},
		ctrl.OpCode(2),
		func(ctx context.Context, commandMessage service.AsyncCommandMessage) {
			commandMessage.NotifyFailed(errors.New("failure"))
		},
	))

	msg := ctrl.NewMessage(uuid.New(), 1, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	assert.Error(t, receivingSvc.InvokeMessageHandlerWithErr(
		context.TODO(),
		&msg,
	), "error while parsing the async commmand: unmarshal error")
}
