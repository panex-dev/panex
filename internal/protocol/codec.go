package protocol

import (
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

func Encode(message Envelope) ([]byte, error) {
	return msgpack.Marshal(message)
}

func DecodeEnvelope(raw []byte) (Envelope, error) {
	var envelope struct {
		V    uint8              `msgpack:"v"`
		T    MessageType        `msgpack:"t"`
		Name MessageName        `msgpack:"name"`
		Src  Source             `msgpack:"src"`
		Data msgpack.RawMessage `msgpack:"data"`
	}
	if err := msgpack.Unmarshal(raw, &envelope); err != nil {
		return Envelope{}, err
	}

	return Envelope{
		V:    envelope.V,
		T:    envelope.T,
		Name: envelope.Name,
		Src:  envelope.Src,
		Data: envelope.Data,
	}, nil
}

func DecodePayload(raw any, out any) error {
	switch typed := raw.(type) {
	case msgpack.RawMessage:
		if err := msgpack.Unmarshal(typed, out); err != nil {
			return fmt.Errorf("unmarshal raw payload: %w", err)
		}
		return nil
	case []byte:
		if err := msgpack.Unmarshal(typed, out); err != nil {
			return fmt.Errorf("unmarshal raw payload: %w", err)
		}
		return nil
	}

	// Compatibility path for tests or callers that still pass typed payload values.
	encoded, err := msgpack.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal typed payload: %w", err)
	}
	if err := msgpack.Unmarshal(encoded, out); err != nil {
		return fmt.Errorf("unmarshal typed payload: %w", err)
	}
	return nil
}
