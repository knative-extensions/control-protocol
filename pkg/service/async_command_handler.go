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

package service

import (
	"context"
	"encoding/binary"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	control "knative.dev/control-protocol/pkg"
)

// Int64CommandId creates a command id from int64
func Int64CommandId(id int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(id))
	return b
}

// AsyncCommandResult is a data structure representing an asynchronous command result.
// This can be used in use cases where the controller sends a command, which is acked as soon as is received by the data plane,
// then the data plane performs an eventually long operation, and when this operation is completed this result is sent back to the control plane.
// This can be used in conjunction with reconciler.AsyncCommandNotificationStore.
type AsyncCommandResult struct {
	// CommandId is the command id
	CommandId []byte
	// Error is the eventual error string. Empty if no error happened
	Error string
}

// IsFailed returns true if this result contains an error
func (k AsyncCommandResult) IsFailed() bool {
	return k.Error != ""
}

func (k AsyncCommandResult) MarshalBinary() (data []byte, err error) {
	bytesNumber := 4 + len(k.CommandId)
	if k.IsFailed() {
		bytesNumber = bytesNumber + 4 + len(k.Error)
	}
	b := make([]byte, bytesNumber)
	binary.BigEndian.PutUint32(b[0:4], uint32(len(k.CommandId)))
	copy(b[4:4+len(k.CommandId)], k.CommandId)
	if k.IsFailed() {
		binary.BigEndian.PutUint32(b[4+len(k.CommandId):4+len(k.CommandId)+4], uint32(len(k.Error)))
		copy(b[4+len(k.CommandId)+4:4+len(k.CommandId)+4+len(k.Error)], k.Error)
	}
	return b, nil
}

func (k *AsyncCommandResult) UnmarshalBinary(data []byte) error {
	commandLength := int(binary.BigEndian.Uint32(data[0:4]))
	k.CommandId = data[4 : 4+commandLength]
	if len(data) > 4+commandLength {
		errLength := int(binary.BigEndian.Uint32(data[4+commandLength : 4+commandLength+4]))
		k.Error = string(data[4+commandLength+4 : 4+commandLength+4+errLength])
	}
	return nil
}

func ParseAsyncCommandResult(data []byte) (interface{}, error) {
	var acr AsyncCommandResult
	if err := acr.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return acr, nil
}

type asyncCommandHandler struct {
	svc         control.Service
	errorOpCode control.OpCode
	handler     func(context.Context, control.ServiceMessage) ([]byte, error)
}

// NewAsyncCommandHandler returns a command handler that wraps the provided handler,
// sending using the provided svc an AsyncCommandResult to the provided resultOpCode opcode with the result of the handler execution.
func NewAsyncCommandHandler(handler func(context.Context, control.ServiceMessage) ([]byte, error), svc control.Service, resultOpCode control.OpCode) control.MessageHandler {
	return &asyncCommandHandler{
		svc:         svc,
		errorOpCode: resultOpCode,
		handler:     handler,
	}
}

func (f *asyncCommandHandler) HandleServiceMessage(ctx context.Context, message control.ServiceMessage) {
	commandId, err := f.handler(ctx, message)

	updateRes := AsyncCommandResult{
		CommandId: commandId,
	}
	if err != nil {
		logging.FromContext(ctx).Warnw("Error while handling the async command", zap.Error(err))
		updateRes.Error = err.Error()
	}

	if err := f.svc.SendAndWaitForAck(f.errorOpCode, updateRes); err != nil {
		logging.FromContext(ctx).Errorw("Error while propagating the update result", zap.Any("asyncCommandResult", updateRes), zap.Error(err))
	}
}
