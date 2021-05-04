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

package network

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

func TestServerTcpConnection_CloseCtxCausesListenerToClose(t *testing.T) {
	ctx, cancelFn := context.WithCancel(context.TODO())
	logger, _ := zap.NewDevelopment()

	listener := &mockListener{
		acceptReturn: make(chan interface{}, 10),
		closeInvoked: atomic.NewBool(false),
	}

	tcpConn := &serverTcpConnection{
		baseTcpConnection: baseTcpConnection{
			ctx:                 ctx,
			logger:              logger.Sugar(),
			writeQueue:          newUnboundedMessageQueue(),
			readQueue:           newUnboundedMessageQueue(),
			unrecoverableErrors: make(chan error, 10),
		},
		listener: listener,
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		// This one should block
		closedCh := make(chan struct{})
		tcpConn.startAcceptPolling(closedCh)
		<-closedCh // This blocks until all polling loops are closed
		wg.Done()
	}()
	go func() {
		// Let's just wait some random time to simulate the connection is actuall doing something
		// (It doesn't make any different to keep or remove this)
		time.Sleep(1 * time.Second)

		cancelFn()
		wg.Done()
	}()

	wg.Wait()

	require.True(t, listener.closeInvoked.Load())
}

type mockListener struct {
	acceptReturn chan interface{}
	closeInvoked *atomic.Bool
}

func (m *mockListener) Accept() (net.Conn, error) {
	val := <-m.acceptReturn
	if err, ok := val.(error); ok {
		return nil, err
	} else {
		// Pass it already closed so it fails if the read is ever invoked
		readReturnCh := make(chan interface{})
		close(readReturnCh)
		return &mockConn{
			readReturn:   readReturnCh,
			closeInvoked: atomic.NewBool(false),
		}, nil
	}
}

func (m *mockListener) Close() error {
	m.acceptReturn <- errors.New("closed connection")
	m.closeInvoked.Store(true)
	return nil
}

func (m *mockListener) Addr() net.Addr {
	return &net.IPAddr{IP: net.IPv4zero}
}
