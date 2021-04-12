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

package reconciler

import (
	"bytes"

	"k8s.io/apimachinery/pkg/types"

	control "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/service"
)

// AsyncCommandNotificationStore is a specialized NotificationStore that is capable to handle service.AsyncCommandResult
type AsyncCommandNotificationStore struct {
	ns *NotificationStore
}

// NewAsyncCommandNotificationStore creates an AsyncCommandNotificationStore
func NewAsyncCommandNotificationStore(enqueueKey func(name types.NamespacedName)) *AsyncCommandNotificationStore {
	return &AsyncCommandNotificationStore{
		ns: &NotificationStore{
			enqueueKey:        enqueueKey,
			payloadParser:     service.ParseAsyncCommandResult,
			notificationStore: make(map[types.NamespacedName]map[string]interface{}),
		},
	}
}

func (notificationStore *AsyncCommandNotificationStore) GetInt64CommandResult(srcName types.NamespacedName, pod string, commandId int64) *service.AsyncCommandResult {
	return notificationStore.GetCommandResult(srcName, pod, service.Int64CommandId(commandId))
}

// GetCommandResult returns the service.AsyncCommandResult when the notification store contains the command result matching srcName, pod and generation
func (notificationStore *AsyncCommandNotificationStore) GetCommandResult(srcName types.NamespacedName, pod string, commandId []byte) *service.AsyncCommandResult {
	val, ok := notificationStore.ns.GetPodNotification(srcName, pod)
	if !ok {
		return nil
	}

	res := val.(service.AsyncCommandResult)

	if !bytes.Equal(res.CommandId, commandId) {
		return nil
	}

	return &res
}

// CleanPodsNotifications is like NotificationStore.CleanPodsNotifications
func (notificationStore *AsyncCommandNotificationStore) CleanPodsNotifications(srcName types.NamespacedName) {
	notificationStore.ns.CleanPodsNotifications(srcName)
}

// CleanPodNotification is like NotificationStore.CleanPodNotification
func (notificationStore *AsyncCommandNotificationStore) CleanPodNotification(srcName types.NamespacedName, pod string) {
	notificationStore.ns.CleanPodNotification(srcName, pod)
}

// MessageHandler is like NotificationStore.MessageHandler
func (notificationStore *AsyncCommandNotificationStore) MessageHandler(srcName types.NamespacedName, pod string) control.MessageHandler {
	return notificationStore.ns.MessageHandler(srcName, pod, PassNewValue)
}
