package network

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestClientPollingLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	initialConn := &mockConn{
		readReturn:         make(chan interface{}),
		closeInvoked:       atomic.NewBool(false),
		closeInvokedSignal: make(chan interface{}),
	}
	dialedConn := &mockConn{
		readReturn:         make(chan interface{}),
		closeInvoked:       atomic.NewBool(false),
		closeInvokedSignal: make(chan interface{}),
	}
	dialer := &mockDialer{
		dialTries: 3,
		conn:      dialedConn,
	}

	tcpConn := newClientTcpConnection(ctx, dialer)
	tcpConn.startPolling(initialConn)

	// Now let's make the initial connection fail
	initialConn.readReturn <- errors.New("Some fancy error")

	// Let's wait for the initial connection to fail
	<-initialConn.closeInvokedSignal

	// At this point, I expect the re-dial to happen behind the scenes.
	// Let's fail it again and wait for the close
	dialedConn.readReturn <- errors.New("Some fancy error in the new conn")
	<-dialedConn.closeInvokedSignal

	// When the unrecoverable errors channel is closed, the tcpConn is being cleaned up
	// but there might be errors here, so let's make sure we read the whole channel and
	// wait for the actual close
	for range tcpConn.unrecoverableErrors {
	}

	require.True(t, dialer.dialTries < 0)
}

type mockDialer struct {
	dialTries int
	conn      *mockConn
}

func (d *mockDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d.dialTries--
	if d.dialTries == 0 {
		return d.conn, nil
	}
	return nil, errors.New("funky error")
}
