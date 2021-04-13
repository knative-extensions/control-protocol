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

package reconciler_test

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"k8s.io/apimachinery/pkg/types"

	control "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/message"
	"knative.dev/control-protocol/pkg/reconciler"
	"knative.dev/control-protocol/pkg/service"
	"knative.dev/control-protocol/pkg/test"
)

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

func (m mockAsyncCommand) SerializedId() []byte {
	return message.Int64CommandId(m.generation)
}

func TestAsyncCommandNotificationStore_Integration_GetCommandResult(t *testing.T) {
	dataPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupAsyncCommandNotificationStoreIntegrationTest(t)

	command1 := &mockAsyncCommand{generation: 1}
	command2 := &mockAsyncCommand{generation: 2}

	require.Nil(t, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, command1))

	require.NoError(t, dataPlane.SendAndWaitForAck(1, message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}))
	require.Equal(t, int32(1), enqueueKeyInvoked.Load())

	require.Nil(t, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, command1))
	require.Equal(t, &message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, command2))
}

func TestAsyncCommandNotificationStore_Integration_CleanPodNotification(t *testing.T) {
	controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupAsyncCommandNotificationStoreIntegrationTest(t)

	command2 := &mockAsyncCommand{generation: 2}

	require.NoError(t, controlPlane.SendAndWaitForAck(1, message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}))
	require.Equal(t, int32(1), enqueueKeyInvoked.Load())

	require.Equal(t, &message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, command2))

	notificationsStore.CleanPodNotification(expectedNamespacedName, expectedPodIp)

	require.Nil(t, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, command2))
}

func TestAsyncCommandNotificationStore_Integration_CleanPodsNotifications(t *testing.T) {
	controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupAsyncCommandNotificationStoreIntegrationTest(t)

	command2 := &mockAsyncCommand{generation: 2}

	require.NoError(t, controlPlane.SendAndWaitForAck(1, message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}))
	require.Equal(t, int32(1), enqueueKeyInvoked.Load())

	require.Equal(t, &message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, command2))

	notificationsStore.CleanPodsNotifications(expectedNamespacedName)

	require.Nil(t, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, command2))
}

func TestAsyncCommand_Integration_Success(t *testing.T) {
	expectedNamespacedName := types.NamespacedName{Namespace: "hello", Name: "world"}
	expectedPodIp := "127.0.0.1"

	_, dataPlane, _, controlPlane := test.MustSetupInsecureControlPair(t)

	commandNotificationStore := reconciler.NewAsyncCommandNotificationStore(func(name types.NamespacedName) {})
	controlPlane.MessageHandler(commandNotificationStore.MessageHandler(expectedNamespacedName, expectedPodIp))

	var wg sync.WaitGroup
	wg.Add(1)

	// The data plane registers an async message handler and handles the command
	dataPlane.MessageHandler(service.NewAsyncCommandHandler(
		dataPlane,
		&mockAsyncCommand{},
		control.OpCode(2),
		func(ctx context.Context, commandMessage service.AsyncCommandMessage) {
			defer wg.Done()
			_, ok := commandMessage.ParsedCommand().(*mockAsyncCommand)
			if assert.True(t, ok) {
				time.Sleep(200 * time.Millisecond)
				commandMessage.NotifySuccess()
			} else {
				commandMessage.NotifyFailed(errors.New("unexpected bad casting"))
			}
		},
	))

	// Control plane sends the command
	require.NoError(t, controlPlane.SendAndWaitForAck(control.OpCode(1), &mockAsyncCommand{generation: 1}))

	wg.Wait()

	// Control plane checks the notification store to see if there's any notification of command done
	commandResult := commandNotificationStore.GetCommandResult(expectedNamespacedName, expectedPodIp, &mockAsyncCommand{generation: 1})
	require.NotNil(t, commandResult)
	require.False(t, commandResult.IsFailed())
	require.Equal(t, message.Int64CommandId(1), commandResult.CommandId)
}

func TestAsyncCommand_Integration_Failure(t *testing.T) {
	expectedNamespacedName := types.NamespacedName{Namespace: "hello", Name: "world"}
	expectedPodIp := "127.0.0.1"

	_, dataPlane, _, controlPlane := test.MustSetupInsecureControlPair(t)

	commandNotificationStore := reconciler.NewAsyncCommandNotificationStore(func(name types.NamespacedName) {})
	controlPlane.MessageHandler(commandNotificationStore.MessageHandler(expectedNamespacedName, expectedPodIp))

	var wg sync.WaitGroup
	wg.Add(1)

	// The data plane registers an async message handler and handles the command
	dataPlane.MessageHandler(service.NewAsyncCommandHandler(
		dataPlane,
		&mockAsyncCommand{},
		control.OpCode(2),
		func(ctx context.Context, commandMessage service.AsyncCommandMessage) {
			defer wg.Done()
			_, ok := commandMessage.ParsedCommand().(*mockAsyncCommand)
			if assert.True(t, ok) {
				time.Sleep(200 * time.Millisecond)
				commandMessage.NotifyFailed(errors.New("failure"))
			} else {
				commandMessage.NotifyFailed(errors.New("unexpected bad casting"))
			}
		},
	))

	// Control plane sends the command
	require.NoError(t, controlPlane.SendAndWaitForAck(control.OpCode(1), &mockAsyncCommand{generation: 1}))

	wg.Wait()

	// Control plane checks the notification store to see if there's any notification of command done
	commandResult := commandNotificationStore.GetCommandResult(expectedNamespacedName, expectedPodIp, &mockAsyncCommand{generation: 1})
	require.NotNil(t, commandResult)
	require.True(t, commandResult.IsFailed())
	require.Equal(t, message.Int64CommandId(1), commandResult.CommandId)
	require.Equal(t, "failure", commandResult.Error)
}

func setupAsyncCommandNotificationStoreIntegrationTest(t *testing.T) (control.Service, *atomic.Int32, *reconciler.AsyncCommandNotificationStore, types.NamespacedName, string) {
	expectedNamespacedName := types.NamespacedName{Namespace: "hello", Name: "world"}
	expectedPodIp := "127.0.0.1"

	_, receiver, _, sender := test.MustSetupInsecureControlPair(t)

	enqueueKeyInvoked := atomic.NewInt32(0)

	notificationsStore := reconciler.NewAsyncCommandNotificationStore(func(name types.NamespacedName) {
		require.Equal(t, expectedNamespacedName, name)
		enqueueKeyInvoked.Inc()
	})

	receiver.MessageHandler(notificationsStore.MessageHandler(expectedNamespacedName, expectedPodIp))

	return sender, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp
}
