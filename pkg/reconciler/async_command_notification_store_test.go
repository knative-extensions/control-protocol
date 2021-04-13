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
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"k8s.io/apimachinery/pkg/types"

	control "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/message"
	"knative.dev/control-protocol/pkg/reconciler"
	"knative.dev/control-protocol/pkg/test"
)

func TestAsyncCommandNotificationStore_Integration_GetCommandResult(t *testing.T) {
	controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupAsyncCommandNotificationStoreIntegrationTest(t)

	require.Nil(t, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, message.Int64CommandId(1)))

	require.NoError(t, controlPlane.SendAndWaitForAck(1, message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}))
	require.Equal(t, int32(1), enqueueKeyInvoked.Load())

	require.Nil(t, notificationsStore.GetInt64CommandResult(expectedNamespacedName, expectedPodIp, 1))
	require.Equal(t, &message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, message.Int64CommandId(2)))
}

func TestAsyncCommandNotificationStore_Integration_CleanPodNotification(t *testing.T) {
	controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupAsyncCommandNotificationStoreIntegrationTest(t)
	require.NoError(t, controlPlane.SendAndWaitForAck(1, message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}))
	require.Equal(t, int32(1), enqueueKeyInvoked.Load())

	require.Equal(t, &message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, message.Int64CommandId(2)))

	notificationsStore.CleanPodNotification(expectedNamespacedName, expectedPodIp)

	require.Nil(t, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, message.Int64CommandId(2)))
}

func TestAsyncCommandNotificationStore_Integration_CleanPodsNotifications(t *testing.T) {
	controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupAsyncCommandNotificationStoreIntegrationTest(t)
	require.NoError(t, controlPlane.SendAndWaitForAck(1, message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}))
	require.Equal(t, int32(1), enqueueKeyInvoked.Load())

	require.Equal(t, &message.AsyncCommandResult{
		CommandId: message.Int64CommandId(2),
		Error:     "",
	}, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, message.Int64CommandId(2)))

	notificationsStore.CleanPodsNotifications(expectedNamespacedName)

	require.Nil(t, notificationsStore.GetCommandResult(expectedNamespacedName, expectedPodIp, message.Int64CommandId(2)))
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
