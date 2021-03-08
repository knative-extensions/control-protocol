package test

type MockPayload string

var SomeMockPayload MockPayload = "Hello world!"

func (m MockPayload) MarshalBinary() (data []byte, err error) {
	return []byte(m), err
}

func (m *MockPayload) UnmarshalBinary(data []byte) error {
	*m = MockPayload(data)
	return nil
}
