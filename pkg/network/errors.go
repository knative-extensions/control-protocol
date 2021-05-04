package network

import (
	"errors"
	"io"
	"net"
	"regexp"
)

func isTransientError(err error) bool {
	// Transient errors are fine
	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() || neterr.Timeout() {
			return true
		}
	}
	return false
}

func isEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

var isUseOfClosedConnectionRegex = regexp.MustCompile("use of closed.* connection")

func isUseOfClosedConnection(err error) bool {
	// Don't rely on this check, it's just used to reduce logging noise, it shouldn't be used as assertion
	return isUseOfClosedConnectionRegex.MatchString(err.Error())
}
