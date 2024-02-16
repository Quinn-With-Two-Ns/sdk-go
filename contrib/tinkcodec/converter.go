package tinkcodec

import (
	"fmt"

	"github.com/tink-crypto/tink-go/v2/tink"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
	"google.golang.org/protobuf/proto"
)

var _ converter.PayloadCodec = (*tinkPayloadCodec)(nil)

const (
	MetadataEncodingEncryptedTink = "binary/encrypted/tink"
)

type tinkPayloadCodec struct {
	aead tink.AEAD
	// Ideally this is some kind of context data that is used to encrypt/decrypt the payload.
	// Ideally workflow ID, but that is too hard to wire up in the Go SDK.
	aad                    []byte
	errorOnUnencryptedData bool
}

// TinkPayloadCodecOptions are the options for creating a new TinkPayloadCodec.
type TinkPayloadCodecOptions struct {
	// Aead is the AEAD primitive to use for encryption and decryption.
	Aead tink.AEAD
	// AssociatedData is the context data that is used to encrypt/decrypt the payload.
	AssociatedData []byte
	// ErrorOnUnencryptedData indicates whether to return an error if unencrypted data is encountered.
	ErrorOnUnencryptedData bool
}

// NewTinkPayloadCodec creates a new TinkPayloadCodec.
func NewTinkPayloadCodec(options TinkPayloadCodecOptions) converter.PayloadCodec {
	return &tinkPayloadCodec{
		aead:                   options.Aead,
		aad:                    options.AssociatedData,
		errorOnUnencryptedData: options.ErrorOnUnencryptedData,
	}
}

// Decode implements converter.PayloadCodec.
func (tpc *tinkPayloadCodec) Decode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	decryptedPayloads := make([]*commonpb.Payload, len(payloads))
	for i, p := range payloads {
		// If the payload is not encrypted, skip decryption.
		payloadMetadata := string(p.Metadata[converter.MetadataEncoding])
		if payloadMetadata != MetadataEncodingEncryptedTink {
			if tpc.errorOnUnencryptedData {
				return nil, fmt.Errorf("payload %s is not encoded with type %s", payloadMetadata, MetadataEncodingEncryptedTink)
			}
			decryptedPayloads[i] = p
		}
		plaintext, err := tpc.aead.Decrypt(p.GetData(), tpc.aad)
		if err != nil {
			return nil, err
		}
		var decryptedPayload commonpb.Payload
		err = proto.Unmarshal(plaintext, &decryptedPayload)
		if err != nil {
			return nil, err
		}
		decryptedPayloads[i] = &decryptedPayload
	}
	return decryptedPayloads, nil
}

// Encode implements converter.PayloadCodec.
func (tpc *tinkPayloadCodec) Encode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	encryptedPayloads := make([]*commonpb.Payload, len(payloads))
	for i, p := range payloads {
		plaintext, err := proto.Marshal(p)
		if err != nil {
			return nil, err
		}
		encryptedPayload, err := tpc.aead.Encrypt(plaintext, tpc.aad)
		if err != nil {
			return nil, err
		}
		encryptedPayloads[i] = &commonpb.Payload{
			Metadata: map[string][]byte{
				converter.MetadataEncoding: []byte(MetadataEncodingEncryptedTink),
			},
			Data: encryptedPayload,
		}
	}
	return encryptedPayloads, nil
}
