package service

import (
	"context"

	control "knative.dev/control-protocol/pkg"
)

type FallibleHandler struct {
	svc         control.Service
	errorOpCode control.OpCode
	handler     func(context.Context, control.ServiceMessage) error
}

func NewFallibleHandler(svc control.Service, errorOpCode control.OpCode, handler func(context.Context, control.ServiceMessage)) control.MessageHandler {
	return &FallibleHandler{
		svc:         svc,
		errorOpCode: errorOpCode,
		handler:     handler,
	}
}

func (f *FallibleHandler) HandleServiceMessage(ctx context.Context, message control.ServiceMessage) {
	if err := f.handler(ctx, message); err != nil {
		// Propagate error

	}

	panic("implement me")
}
