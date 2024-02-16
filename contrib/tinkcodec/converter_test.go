package tinkcodec_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/testing/fakekms"
	"github.com/tink-crypto/tink-go/v2/tink"
	"go.temporal.io/sdk/contrib/tinkcodec"
	"go.temporal.io/sdk/converter"
)

// The fake KMS should only be used in tests. It is not secure.
const keyURI = "fake-kms://CM2b3_MDElQKSAowdHlwZS5nb29nbGVhcGlzLmNvbS9nb29nbGUuY3J5cHRvLnRpbmsuQWVzR2NtS2V5EhIaEIK75t5L-adlUwVhWvRuWUwYARABGM2b3_MDIAE"

type TinkCodecTestSuite struct {
	suite.Suite
	aead tink.AEAD
}

func (tc *TinkCodecTestSuite) SetupTest() {
	client, err := fakekms.NewClient(keyURI)
	tc.NoError(err)

	kekAEAD, err := client.GetAEAD(keyURI)
	tc.NoError(err)

	tc.aead = aead.NewKMSEnvelopeAEAD2(aead.AES256GCMKeyTemplate(), kekAEAD)
}

func (tc *TinkCodecTestSuite) TestCodec() {
	pc := tinkcodec.NewTinkPayloadCodec(tinkcodec.TinkPayloadCodecOptions{
		Aead:           tc.aead,
		AssociatedData: []byte("workflow-id"),
	})
	cdc := converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), pc)
	ep, err := cdc.ToPayload("test data")
	tc.NoError(err)
	tc.NotNil(ep)
	tc.Equal("binary/encrypted/tink", string(ep.Metadata[converter.MetadataEncoding]))

	var plaintext string
	tc.NoError(cdc.FromPayload(ep, &plaintext))
	tc.Equal("test data", plaintext)
}

func (tc *TinkCodecTestSuite) TestAssociatedData() {
	pc := tinkcodec.NewTinkPayloadCodec(tinkcodec.TinkPayloadCodecOptions{
		Aead:           tc.aead,
		AssociatedData: []byte("workflow-id"),
	})
	cdc := converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), pc)
	ep, err := cdc.ToPayload("test data")
	tc.NoError(err)
	tc.NotNil(ep)
	// Create a new codec with a different associated data.
	pc = tinkcodec.NewTinkPayloadCodec(tinkcodec.TinkPayloadCodecOptions{
		Aead:           tc.aead,
		AssociatedData: []byte("workflow-id-2"),
	})
	cdc = converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), pc)
	// Try to decode the payload with the wrong associated data.
	var plaintext string
	tc.Error(cdc.FromPayload(ep, &plaintext))
}

func (tc *TinkCodecTestSuite) TestErrorOnUnencryptedData() {
	pc := tinkcodec.NewTinkPayloadCodec(tinkcodec.TinkPayloadCodecOptions{
		Aead:                   tc.aead,
		AssociatedData:         []byte("workflow-id"),
		ErrorOnUnencryptedData: true,
	})
	cdc := converter.NewCodecDataConverter(converter.GetDefaultDataConverter(), pc)
	ep, err := cdc.ToPayload("test data")
	tc.NoError(err)
	tc.NotNil(ep)
	tc.Equal("binary/encrypted/tink", string(ep.Metadata[converter.MetadataEncoding]))
	// Corrupt the metadata to make it look like the payload is not encrypted.
	ep.Metadata[converter.MetadataEncoding] = []byte("binary/encrypted/other")
	var plaintext string
	tc.ErrorContains(cdc.FromPayload(ep, &plaintext), "payload binary/encrypted/other is not encoded with type binary/encrypted/tink")
}

func TestTinkCodecTestSuite(t *testing.T) {
	suite.Run(t, new(TinkCodecTestSuite))
}
