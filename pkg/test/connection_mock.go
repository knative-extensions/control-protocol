package test

import control "knative.dev/control-protocol/pkg"

type ConnectionMock struct {
	OutboundCh chan *control.OutboundMessage
	InboundCh  chan *control.InboundMessage
	ErrorsCh   chan error
}

func NewConnectionMock() *ConnectionMock {
	return &ConnectionMock{
		OutboundCh: make(chan *control.OutboundMessage, 10),
		InboundCh:  make(chan *control.InboundMessage, 10),
		ErrorsCh:   make(chan error, 10),
	}
}

func (c *ConnectionMock) OutboundMessages() chan<- *control.OutboundMessage {
	return c.OutboundCh
}

func (c *ConnectionMock) InboundMessages() <-chan *control.InboundMessage {
	return c.InboundCh
}

func (c *ConnectionMock) Errors() <-chan error {
	return c.ErrorsCh
}
