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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap"

	ctrl "knative.dev/control-protocol/pkg"
)

func TestBaseTcpConnection_ConsumeConnection_ReturnsAfterConnectionFailure(t *testing.T) {
	ctx, cancelFn := context.WithCancel(context.TODO())
	logger, _ := zap.NewDevelopment()

	conn := &mockConn{
		readReturn:         make(chan interface{}, 10),
		closeInvoked:       atomic.NewBool(false),
		closeInvokedSignal: make(chan interface{}),
	}

	tcpConn := &baseTcpConnection{
		ctx:                 ctx,
		logger:              logger.Sugar(),
		writeQueue:          newUnboundedMessageQueue(),
		readQueue:           newUnboundedMessageQueue(),
		unrecoverableErrors: make(chan error, 10),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		// This one should block
		tcpConn.consumeConnection(conn)
		wg.Done()
	}()
	go func() {
		// Let's just wait some random time to simulate the connection is actuall doing something
		// (It doesn't make any different to keep or remove this)
		time.Sleep(1 * time.Second)

		conn.readReturn <- errors.New("something broke badly!")
		wg.Done()
	}()

	wg.Wait()

	require.True(t, conn.closeInvoked.Load())

	cancelFn()
}

func TestBaseTcpConnection_ConsumeConnection_ReturnsAfterContextClosed(t *testing.T) {
	ctx, cancelFn := context.WithCancel(context.TODO())
	logger, _ := zap.NewDevelopment()

	conn := &mockConn{
		readReturn:         make(chan interface{}, 10),
		closeInvokedSignal: make(chan interface{}),
		closeInvoked:       atomic.NewBool(false),
	}

	tcpConn := &baseTcpConnection{
		ctx:                 ctx,
		logger:              logger.Sugar(),
		writeQueue:          newUnboundedMessageQueue(),
		readQueue:           newUnboundedMessageQueue(),
		unrecoverableErrors: make(chan error, 10),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		// This one should block
		tcpConn.consumeConnection(conn)
		wg.Done()
	}()
	go func() {
		// Let's just wait some random time to simulate the connection is actually doing something
		// (It doesn't make any different to keep or remove this)
		time.Sleep(1 * time.Second)

		cancelFn()
		wg.Done()
	}()

	wg.Wait()

	require.True(t, conn.closeInvoked.Load())
}

func TestBaseTcpConnection_ConsumeConnection_BrokenConnectionDoesntLoseOutboundMessage(t *testing.T) {
	ctx, cancelFn := context.WithCancel(context.TODO())
	logger, _ := zap.NewDevelopment()

	conn := &mockConn{
		readReturn:         make(chan interface{}, 10),
		writeReturn:        make(chan interface{}),
		closeInvoked:       atomic.NewBool(false),
		closeInvokedSignal: make(chan interface{}),
	}

	tcpConn := &baseTcpConnection{
		ctx:                 ctx,
		logger:              logger.Sugar(),
		writeQueue:          newUnboundedMessageQueue(),
		readQueue:           newUnboundedMessageQueue(),
		unrecoverableErrors: make(chan error, 10),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	msg := ctrl.NewMessage(uuid.New(), 10, nil)

	go func() {
		// This one should block
		tcpConn.consumeConnection(conn)
		wg.Done()
	}()
	go func() {
		// Let's have some fun here and fail the writes
		tcpConn.WriteMessage(&msg)
		// This blocks while waiting for writeReturn to be read
		conn.writeReturn <- errors.New("something broke while writing")

		msgInTheCh := tcpConn.writeQueue.blockingPoll(ctx)
		assert.Same(t, &msg, msgInTheCh)

		// Close the conn
		cancelFn()
		wg.Done()
	}()

	wg.Wait()
}

type mockConn struct {
	writeReturn        chan interface{}
	readReturn         chan interface{}
	closeInvoked       *atomic.Bool
	closeInvokedSignal chan interface{}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	select {
	case val := <-m.readReturn:
		if err, ok := val.(error); ok {
			return 0, err
		} else {
			return len(b), nil
		}
	case <-m.closeInvokedSignal:
		return 0, errors.New("closed connection")
	}
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	select {
	case val := <-m.writeReturn:
		if err, ok := val.(error); ok {
			return 0, err
		} else {
			return len(b), nil
		}
	case <-m.closeInvokedSignal:
		return 0, errors.New("closed connection")
	}
}

func (m *mockConn) Close() error {
	if m.closeInvoked != nil {
		m.closeInvoked.Store(true)
	}
	close(m.closeInvokedSignal)
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.IPAddr{IP: net.IPv4zero}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.IPAddr{IP: net.IPv4zero}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}
