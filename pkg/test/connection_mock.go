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
	"sync"

	control "knative.dev/control-protocol/pkg"
)

type ConnectionMock struct {
	outboundMessageCond *sync.Cond
	outboundMessages    []*control.Message

	inboundCh chan *control.Message
	ErrorsCh  chan error
}

var _ control.Connection = (*ConnectionMock)(nil)

func NewConnectionMock() *ConnectionMock {
	return &ConnectionMock{
		outboundMessageCond: sync.NewCond(new(sync.Mutex)),
		inboundCh:           make(chan *control.Message, 10),
		ErrorsCh:            make(chan error, 10),
	}
}

func (c *ConnectionMock) WriteMessage(ctx context.Context, msg *control.Message) error {
	c.outboundMessageCond.L.Lock()
	c.outboundMessages = append(c.outboundMessages, msg)
	c.outboundMessageCond.L.Unlock()
	c.outboundMessageCond.Broadcast()
	return nil
}

func (c *ConnectionMock) ReadMessage(ctx context.Context) (*control.Message, error) {
	select {
	case msg, ok := <-c.inboundCh:
		if !ok {
			return nil, ctx.Err()
		}
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *ConnectionMock) Errors() <-chan error {
	return c.ErrorsCh
}

func (c *ConnectionMock) PushInboundMessage(msg *control.Message) {
	c.inboundCh <- msg
}

func (c *ConnectionMock) WaitAtLeastOneOutboundMessage() []*control.Message {
	c.outboundMessageCond.L.Lock()
	for len(c.outboundMessages) == 0 {
		c.outboundMessageCond.Wait()
	}
	cpy := make([]*control.Message, len(c.outboundMessages))
	copy(cpy, c.outboundMessages)
	c.outboundMessageCond.L.Unlock()
	return cpy
}
