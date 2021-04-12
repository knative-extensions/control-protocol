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

package test

import (
	"context"
	"encoding"
	"fmt"
	"sync"

	"go.uber.org/atomic"

	control "knative.dev/control-protocol/pkg"
)

type ServiceMock struct {
	ackFuncs map[control.OpCode]func()

	messageHandler control.MessageHandler
	errorHandler   control.ErrorHandler
}

func NewServiceMock() *ServiceMock {
	return &ServiceMock{
		ackFuncs: make(map[control.OpCode]func()),
	}
}

func (s *ServiceMock) SendAndWaitForAck(opcode control.OpCode, payload encoding.BinaryMarshaler) error {
	_, err := payload.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("MarshalBinary should not panic: %v", err))
	}

	var wg sync.WaitGroup
	wg.Add(1)
	s.ackFuncs[opcode] = wg.Done
	wg.Wait()

	return nil
}

func (s *ServiceMock) MessageHandler(handler control.MessageHandler) {
	s.messageHandler = handler
}

func (s *ServiceMock) ErrorHandler(handler control.ErrorHandler) {
	s.errorHandler = handler
}

// InvokeMessageHandler invokes the registered message handler and returns true if the message was acked back
func (s *ServiceMock) InvokeMessageHandler(ctx context.Context, message *control.Message) bool {
	acked := atomic.NewBool(false)
	ackFn := func(err error) {
		acked.Store(true)
	}

	s.messageHandler.HandleServiceMessage(ctx, control.NewServiceMessage(message, ackFn))
	return acked.Load()
}

func (s *ServiceMock) InvokeErrorHandler(ctx context.Context, err error) {
	s.errorHandler.HandleServiceError(ctx, err)
}

// AckIt propagates the ack for the last message sent using the provided opcode
func (s *ServiceMock) AckIt(code control.OpCode) {
	s.ackFuncs[code]()
}
