package test_images

import control "knative.dev/control-protocol/pkg"

const (
	AcceptMessage = control.OpCode(0)
	FailMessage   = control.OpCode(1)
	DoneMessage   = control.OpCode(2)

	ErrorAck = "expected error"
)

type BinaryPayload []byte

func (b BinaryPayload) MarshalBinary() (data []byte, err error) {
	return b, nil
}
