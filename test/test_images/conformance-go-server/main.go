package main

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"os"
	"strconv"
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

	devLogger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	logger := devLogger.Sugar()
	ctx := logging.WithLogger(context.Background(), logger)

	ctrlServer, err := network.StartInsecureControlServer(ctx, network.WithPort(port))
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
