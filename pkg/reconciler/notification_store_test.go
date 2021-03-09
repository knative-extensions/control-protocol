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
	"knative.dev/control-protocol/pkg/reconciler"
	"knative.dev/control-protocol/pkg/test"
)

func TestNotificationStore_Integration_StoreMessages(t *testing.T) {
	controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupNotificationStoreIntegrationTest(t, test.MockMessageMerger)

	require.NoError(t, controlPlane.SendAndWaitForAck(1, test.MockPayload("Funky!")))
	require.NoError(t, controlPlane.SendAndWaitForAck(1, test.MockPayload("More Funky!")))

	require.Equal(t, int32(2), enqueueKeyInvoked.Load())

	stored, ok := notificationsStore.GetPodNotification(expectedNamespacedName, expectedPodIp)
	require.True(t, ok)

	value := stored.(*test.MockPayload)
	require.Equal(t, "Funky!More Funky!", string(*value))
}

func TestNotificationStore_Integration_DontReconcileTwice(t *testing.T) {
	controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupNotificationStoreIntegrationTest(t, reconciler.PassNewValue)

	require.NoError(t, controlPlane.SendAndWaitForAck(1, test.MockPayload("Funky!")))
	require.NoError(t, controlPlane.SendAndWaitForAck(1, test.MockPayload("Funky!")))

	require.Equal(t, int32(1), enqueueKeyInvoked.Load())

	stored, ok := notificationsStore.GetPodNotification(expectedNamespacedName, expectedPodIp)
	require.True(t, ok)

	value := stored.(*test.MockPayload)
	require.Equal(t, "Funky!", string(*value))
}

func TestNotificationStore_Integration_GetMessages(t *testing.T) {
	controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupNotificationStoreIntegrationTest(t, test.MockMessageMerger)

	stored, ok := notificationsStore.GetPodNotification(expectedNamespacedName, expectedPodIp)
	require.False(t, ok)
	require.Nil(t, stored)

	stored, ok = notificationsStore.GetPodsNotifications(expectedNamespacedName)
	require.False(t, ok)
	require.Nil(t, stored)

	require.NoError(t, controlPlane.SendAndWaitForAck(1, test.MockPayload("Funky!")))

	require.Equal(t, int32(1), enqueueKeyInvoked.Load())

	stored, ok = notificationsStore.GetPodsNotifications(expectedNamespacedName)
	require.True(t, ok)
	require.Len(t, stored, 1)

	stored, ok = notificationsStore.GetPodNotification(expectedNamespacedName, expectedPodIp)
	require.True(t, ok)
	value := stored.(*test.MockPayload)
	require.Equal(t, "Funky!", string(*value))
}

func TestNotificationStore_Integration_ClearMessagesWithoutAny(t *testing.T) {
	_, _, notificationsStore, expectedNamespacedName, expectedPodIp := setupNotificationStoreIntegrationTest(t, test.MockMessageMerger)

	notificationsStore.CleanPodNotification(expectedNamespacedName, expectedPodIp)
	notificationsStore.CleanPodsNotifications(expectedNamespacedName)
}

func TestNotificationStore_Integration_ClearMessages(t *testing.T) {
	controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp := setupNotificationStoreIntegrationTest(t, test.MockMessageMerger)

	require.NoError(t, controlPlane.SendAndWaitForAck(1, test.MockPayload("Funky!")))

	require.Equal(t, int32(1), enqueueKeyInvoked.Load())

	stored, ok := notificationsStore.GetPodNotification(expectedNamespacedName, expectedPodIp)
	require.True(t, ok)
	value := stored.(*test.MockPayload)
	require.Equal(t, "Funky!", string(*value))

	notificationsStore.CleanPodNotification(expectedNamespacedName, expectedPodIp)

	stored, ok = notificationsStore.GetPodsNotifications(expectedNamespacedName)
	require.False(t, ok)
	require.Nil(t, stored)
}

func setupNotificationStoreIntegrationTest(t *testing.T, merger reconciler.ValueMerger) (control.Service, *atomic.Int32, *reconciler.NotificationStore, types.NamespacedName, string) {
	expectedNamespacedName := types.NamespacedName{Namespace: "hello", Name: "world"}
	expectedPodIp := "127.0.0.1"

	_, dataPlane, _, controlPlane := test.MustSetupInsecureControlPair(t)

	enqueueKeyInvoked := atomic.NewInt32(0)

	notificationsStore := reconciler.NewNotificationStore(func(name types.NamespacedName) {
		require.Equal(t, expectedNamespacedName, name)
		enqueueKeyInvoked.Inc()
	}, test.ParseMockMessage)

	dataPlane.MessageHandler(notificationsStore.ControlMessageHandler(expectedNamespacedName, expectedPodIp, merger))

	return controlPlane, enqueueKeyInvoked, notificationsStore, expectedNamespacedName, expectedPodIp
}
