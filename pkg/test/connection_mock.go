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

func (c *ConnectionMock) ConsumeOutboundMessages() []*control.OutboundMessage {
	var res []*control.OutboundMessage
	for {
		select {
		case msg, ok := <-c.OutboundCh:
			if !ok {
				return res
			}
			res = append(res, msg)
		default:
			return res
		}
	}
}
