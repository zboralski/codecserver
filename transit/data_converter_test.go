package transit

import (
	"context"
	"testing"

	"github.com/hashicorp/vault-client-go"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/converter"
)

func Test_Codec(t *testing.T) {
	client, err := vault.New(vault.WithEnvironment())
	require.NoError(t, err)

	defaultDc := converter.GetDefaultDataConverter()
	defaultPayloads, err := defaultDc.ToPayloads("Testing")
	require.NoError(t, err)

	codec := &Codec{Client: client, KeyID: "default"}

	encrypted, err := codec.Encode(defaultPayloads.Payloads)
	require.NoError(t, err)
	require.NotEqual(t, defaultPayloads.Payloads[0].GetData(), encrypted[0].GetData())

	decrypted, err := codec.Decode(encrypted)
	require.NoError(t, err)
	require.Equal(t, defaultPayloads.Payloads[0].GetData(), decrypted[0].GetData())
}

func Test_DataConverter(t *testing.T) {
	defaultDc := converter.GetDefaultDataConverter()

	ctx := context.Background()
	ctx = context.WithValue(ctx, PropagateKey, CryptContext{KeyID: "default"})

	client, err := vault.New(vault.WithEnvironment())
	require.NoError(t, err)

	cryptDc := NewEncryptionDataConverter(
		client,
		converter.GetDefaultDataConverter(),
		DataConverterOptions{},
	)
	cryptDcWc := cryptDc.WithContext(ctx, client)

	defaultPayloads, err := defaultDc.ToPayloads("Testing")
	require.NoError(t, err)

	encryptedPayloads, err := cryptDcWc.ToPayloads("Testing")
	require.NoError(t, err)

	require.NotEqual(t, defaultPayloads.Payloads[0].GetData(), encryptedPayloads.Payloads[0].GetData())

	var result string
	err = cryptDc.FromPayloads(encryptedPayloads, &result)
	require.NoError(t, err)

	require.Equal(t, "Testing", result)
}
