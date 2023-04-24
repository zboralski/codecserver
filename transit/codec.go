package transit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"

	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

type Codec struct {
	Client *vault.Client
	KeyID  string
}

// Encode implements converter.PayloadCodec.Encode.
func (e *Codec) Encode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	ctx := context.Background()
	result := make([]*commonpb.Payload, len(payloads))

	for i, p := range payloads {
		plaintext, err := p.Marshal()
		if err != nil {
			return nil, fmt.Errorf("error marshaling payload: %w", err)
		}

		req := schema.NewTransitEncryptRequestWithDefaults()
		req.Plaintext = base64.StdEncoding.EncodeToString(plaintext)

		// Encrypt the payload using Vault's TransitEncrypt
		resp, err := e.Client.Secrets.TransitEncrypt(ctx, e.KeyID, *req)
		if err != nil {
			return nil, fmt.Errorf("error encrypting: %w", err)
		}

		ciphertext, ok := resp.Data["ciphertext"].(string)
		if !ok {
			return nil, fmt.Errorf("no ciphertext returned")
		}

		keyVersion, ok := resp.Data["key_version"].(json.Number)
		if !ok {
			return nil, fmt.Errorf("no key_version returned")
		}

		// Create a commonpb.Payload with ciphertext and metadata; add to result
		result[i] = &commonpb.Payload{
			Metadata: map[string][]byte{
				converter.MetadataEncoding:   []byte(MetadataEncodingEncrypted),
				MetadataEncryptionKeyVersion: []byte(keyVersion.String()),
				MetadataEncryptionKeyID:      []byte(e.KeyID),
			},
			Data: []byte(ciphertext),
		}
	}

	return result, nil
}

// Decode implements converter.PayloadCodec.Decode.
func (e *Codec) Decode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	ctx := context.Background()
	result := make([]*commonpb.Payload, len(payloads))

	for i, p := range payloads {
		// If payload isn't encrypted, add to result unchanged
		if string(p.Metadata[converter.MetadataEncoding]) != MetadataEncodingEncrypted {
			result[i] = p
			continue
		}

		keyID, ok := p.Metadata[MetadataEncryptionKeyID]
		if !ok {
			return nil, fmt.Errorf("no encryption key id")
		}

		req := schema.NewTransitDecryptRequestWithDefaults()
		req.Ciphertext = string(p.Data)

		// Decrypt payload using Vault's TransitDecrypt
		resp, err := e.Client.Secrets.TransitDecrypt(ctx, string(keyID), *req)
		if err != nil {
			return nil, fmt.Errorf("error decrypting: %w", err)
		}

		encoded, ok := resp.Data["plaintext"].(string)
		if !ok {
			return nil, fmt.Errorf("no plaintext returned")
		}

		plaintext, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("error decoding base64 plaintext: %w", err)
		}

		// Convert decrypted plaintext into commonpb.Payload; add to result
		result[i] = &commonpb.Payload{}
		if err = result[i].Unmarshal([]byte(plaintext)); err != nil {
			return nil, err
		}
	}

	return result, nil
}
