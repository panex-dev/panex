package protocol

import "github.com/vmihailenco/msgpack/v5"

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
		return err
	}

	return msgpack.Unmarshal(encoded, out)
}
