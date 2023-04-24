package transit

import (
	"context"

	"github.com/hashicorp/vault-client-go"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

const (
	MetadataEncodingEncrypted    = "binary/encrypted"
	MetadataEncryptionKeyID      = "encryption-key-id"
	MetadataEncryptionKeyVersion = "encryption-key-version"
)

type DataConverter struct {
	// Until EncodingDataConverter supports workflow.ContextAware
	parent converter.DataConverter
	converter.DataConverter
	options DataConverterOptions
}

type DataConverterOptions struct {
	KeyID string

	// Enable ZLib compression before encryption.
	Compress bool
}

// TODO: Implement workflow.ContextAware in CodecDataConverter
// Note that you only need to implement this function if you need to vary the encryption KeyID per workflow.
// func (dc *DataConverter) WithWorkflowContext(ctx workflow.Context) converter.DataConverter {
// 	if val, ok := ctx.Value(PropagateKey).(CryptContext); ok {
// 		parent := dc.parent
// 		if parentWithContext, ok := parent.(workflow.ContextAware); ok {
// 			parent = parentWithContext.WithWorkflowContext(ctx)
// 		}

// 		options := dc.options
// 		options.KeyID = val.KeyID

// 		return NewEncryptionDataConverter(parent, options)
// 	}

// 	return dc
// }

// TODO: Implement workflow.ContextAware in EncodingDataConverter
// Note that you only need to implement this function if you need to vary the encryption KeyID per workflow.
func (dc *DataConverter) WithContext(ctx context.Context, client *vault.Client) converter.DataConverter {
	if val, ok := ctx.Value(PropagateKey).(CryptContext); ok {
		parent := dc.parent
		if parentWithContext, ok := parent.(workflow.ContextAware); ok {
			parent = parentWithContext.WithContext(ctx)
		}

		options := dc.options
		options.KeyID = val.KeyID

		return NewEncryptionDataConverter(client, parent, options)
	}

	return dc
}

// NewEncryptionDataConverter creates a new instance of EncryptionDataConverter wrapping a DataConverter
func NewEncryptionDataConverter(client *vault.Client, dataConverter converter.DataConverter, options DataConverterOptions) *DataConverter {
	codecs := []converter.PayloadCodec{
		&Codec{Client: client, KeyID: options.KeyID},
	}
	// // Enable compression if requested.
	// // Note that this must be done before encryption to provide any value. Encrypted data should by design not compress very well.
	// // This means the compression codec must come after the encryption codec here as codecs are applied last -> first.
	if options.Compress {
		codecs = append(codecs, converter.NewZlibCodec(converter.ZlibCodecOptions{AlwaysEncode: true}))
	}

	return &DataConverter{
		parent:        dataConverter,
		DataConverter: converter.NewCodecDataConverter(dataConverter, codecs...),
		options:       options,
	}
}
