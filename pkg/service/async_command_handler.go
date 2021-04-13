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

	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	control "knative.dev/control-protocol/pkg"
	message2 "knative.dev/control-protocol/pkg/message"
)

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

	updateRes := message2.AsyncCommandResult{
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
