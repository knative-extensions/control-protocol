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

package service_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	control "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/service"
	"knative.dev/control-protocol/pkg/test"
)

func TestCachingService(t *testing.T) {
	mockConnection := test.NewConnectionMock()

	ctx, cancelFn := context.WithCancel(context.TODO())
	t.Cleanup(cancelFn)

	svc := service.WithCachingService(ctx)(service.NewService(ctx, mockConnection))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			assert.NoError(t, svc.SendAndWaitForAck(10, test.SomeMockPayload))
		}
	}()

	outboundMessage := <-mockConnection.OutboundCh
	require.Equal(t, uint8(10), outboundMessage.OpCode())
	require.Equal(t, uint32(len([]byte(test.SomeMockPayload))), outboundMessage.Length())

	inboundMessage := control.NewMessage(outboundMessage.UUID(), uint8(control.AckOpCode), nil)
	mockConnection.InboundCh <- &inboundMessage

	wg.Wait()

	require.Empty(t, mockConnection.ConsumeOutboundMessages()) // No more outbound messages, just one
}
