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

package message_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"knative.dev/control-protocol/pkg/message"
)

func TestAsyncCommandResult_RoundTrip(t *testing.T) {
	testCases := map[string]message.AsyncCommandResult{
		"positive generation": {
			CommandId: message.Int64CommandId(1),
		},
		"negative generation": {
			CommandId: message.Int64CommandId(-1),
		},
		"with error": {
			CommandId: message.Int64CommandId(1),
			Error:     "funky error",
		},
	}

	for testName, commandResult := range testCases {
		t.Run(testName, func(t *testing.T) {
			marshalled, err := commandResult.MarshalBinary()
			require.NoError(t, err)

			expectedLength := 4 + len(commandResult.CommandId)
			if commandResult.IsFailed() {
				expectedLength = expectedLength + 4 + len(commandResult.Error)
			}
			require.Len(t, marshalled, expectedLength)

			have, err := message.ParseAsyncCommandResult(marshalled)
			require.NoError(t, err)
			require.Equal(t, commandResult, have)
		})
	}
}
