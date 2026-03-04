package protocol

import (
	"fmt"

	"github.com/vmihailenco/msgpack/v5"
)

func Encode(message Envelope) ([]byte, error) {
	return msgpack.Marshal(message)
}

func DecodeEnvelope(raw []byte) (Envelope, error) {
	var envelope Envelope
	if err := msgpack.Unmarshal(raw, &envelope); err != nil {
		return Envelope{}, err
	}

	return envelope, nil
}

func DecodePayload(raw any, out any) error {
	encoded, err := msgpack.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal payload for re-decode: %w", err)
	}
	if err := msgpack.Unmarshal(encoded, out); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	return nil
}
