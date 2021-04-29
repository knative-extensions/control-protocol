/*
 * Copyright 2021 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"
	ctrl "knative.dev/control-protocol/pkg"
	"knative.dev/control-protocol/pkg/network"
	ctrlservice "knative.dev/control-protocol/pkg/service"
	"knative.dev/control-protocol/test/test_images"
	"knative.dev/pkg/logging"
)

func main() {
	portStr := os.Getenv("PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(fmt.Sprintf("cannot parse the PORT env: %v", err))
	}

	tlsEnv := strings.TrimSpace(strings.ToLower(os.Getenv("TLS")))

	devLogger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	logger := devLogger.Sugar()
	ctx := logging.WithLogger(context.Background(), logger)

	var ctrlServer *network.ControlServer
	if tlsEnv == "true" {
		ctrlServer, err = network.StartControlServer(ctx, network.LoadServerTLSConfigFromFile, network.WithPort(port))
	} else {
		ctrlServer, err = network.StartInsecureControlServer(ctx, network.WithPort(port))
	}
	if err != nil {
		panic(err)
	}

	logger.Info("Started control server")

	var doneWg sync.WaitGroup
	doneWg.Add(1)

	var receivedMessages []ctrl.ServiceMessage
	ctrlServer.MessageHandler(ctrlservice.MessageRouter{
		test_images.AcceptMessage: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			logger.Infof("Received accept message with payload length %d", len(message.Payload()))

			receivedMessages = append(receivedMessages, message)
			message.Ack()
		}),
		test_images.FailMessage: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			logger.Infof("Received fail message with payload length %d", len(message.Payload()))

			receivedMessages = append(receivedMessages, message)
			message.AckWithError(errors.New(test_images.ErrorAck))
		}),
		test_images.DoneMessage: ctrl.MessageHandlerFunc(func(ctx context.Context, message ctrl.ServiceMessage) {
			logger.Infof("Received done message with payload length %d", len(message.Payload()))

			receivedMessages = append(receivedMessages, message)
			message.Ack()
			doneWg.Done()
		}),
	})

	// Wait for the done message
	doneWg.Wait()

	// Ready to start sending back
	for _, msg := range receivedMessages {
		opCode := ctrl.OpCode(msg.Headers().OpCode())
		var payload encoding.BinaryMarshaler
		if len(msg.Payload()) != 0 {
			payload = test_images.BinaryPayload(msg.Payload())
		}
		err := ctrlServer.SendAndWaitForAck(
			opCode,
			payload,
		)
		logger.Infof("Sent message with opcode %d and payload length %d", msg.Headers().OpCode(), len(msg.Payload()))

		if ctrl.OpCode(msg.Headers().OpCode()) == test_images.FailMessage && err == nil {
			panic("Expecting error")
		} else if ctrl.OpCode(msg.Headers().OpCode()) != test_images.FailMessage && err != nil {
			panic(err)
		}
	}

	// We're done!
	logger.Info("Test passed")
}
