# Temporal Codec Server

This repository provides a solution for integrating the Transit Secrets Engine from HashiCorp's Vault with Temporal.

The Transit Secrets Engine, provided by HashiCorp's Vault, delivers cryptographic services for data in transit. It functions as "cryptography as a service" or "encryption as a service" and handles tasks such as encryption, signing and verifying data, generating data hashes and HMACs, and supplying random bytes.

The main purpose of Transit is to encrypt application data while storing encrypted data in a primary data store. This approach eases the encryption/decryption burden on application developers and places it on Vault operators.

## NIST Rotation Guidance

Periodic rotation of the encryption keys is recommended, even in the absence of compromise. For AES-GCM keys, rotation should occur before approximately 2^32 encryptions have been performed by a key version, following the guidelines of NIST publication 800-38D. It is recommended that operators estimate the encryption rate of a key and use that to determine a frequency of rotation that prevents the guidance limits from being reached. For example, if one determines that the estimated rate is 40 million operations per day, then rotating a key every three months is sufficient.

The Vault Transit secrets engine enables simple encryption key rotation. Key rotation can be performed manually or automatically through an API endpoint via cron, a CI pipeline, or a Temporal workflow.

Vault manages a versioned keyring, allowing the admin to determine the minimum decryption version. When data is encrypted with Vault, the key version used for encryption is added to the beginning of the ciphertext.

## Rotating the key

The key rotation process in the Transit secrets engine is entirely transparent to the Temporal codec implementation because the ciphertext is prefixed with the key version. This approach ensures seamless operation during key rotation without affecting the codec implementation.

To rotate the underlying encryption key, generate a new encryption key and add it to the keyring for the specified key:

```bash
vault write -f transit/keys/default/rotate
```

## References
- [Transit Secrets Engine](https://developer.hashicorp.com/vault/docs/secrets/transit)
- [Encryption as a Service: Transit Secrets Engine](https://developer.hashicorp.com/vault/tutorials/encryption-as-a-service/eaas-transit)
- [codec-server go sample](https://github.com/temporalio/samples-go/tree/main/codec-server)
- [encryption go sample](https://github.com/temporalio/samples-go/tree/main/encryption)

## Setup

1. Set up a vault development server

```bash
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=root
export VAULT_DEV_ROOT_TOKEN_ID=$VAULT_TOKEN

vault server -dev
```

### Enable the Transit secrets engine

```bash
vault secrets enable transit
```

### Map keys to namespaces

```bash
vault write -f transit/keys/default
```

### Test that the encryption is working

```bash
go test
PASS
ok      github.com/zboralski/codecserver        0.045s
```

### Configuring the Temporal client

```go
func NewClient(options client.Options) (client.Client, error) {
    vaultClient, err := vault.New(vault.WithEnvironment())
    if err != nil {
      return nil, err
    }

    if options.HostPort == "" {
      options.HostPort = os.Getenv("TEMPORAL_GRPC_ENDPOINT")
    }

    if options.Logger == nil {
      options.Logger = zap.NewLogger()
    }

    options.DataConverter = codecserver.NewEncryptionDataConverter(
        vaultClient,
        converter.GetDefaultDataConverter(),
        codecserver.DataConverterOptions{Compress: true, KeyID: namespace},
    )
    options.ContextPropagators = []workflow.ContextPropagator{codecserver.NewContextPropagator()}

    return client.NewClient(options)
}
```

### Start the codec server

The `CORS_ORIGIN` should be set to the Temporal UI URL.

The `TLS_CERT_FILE` and `TLS_KEY_FILE` environment variables are used to enable Transport Layer Security (TLS) encryption for the codecserver application. TLS is a cryptographic protocol that provides secure communication over the internet.

To enable TLS, you need to set the `TLS_CERT_FILE` and `TLS_KEY_FILE` environment variables to the paths of the certificate and key files, respectively. The certificate file contains the public key, and the key file contains the private key that are used to establish a secure connection.

Assuming you have the certificate and key files ready, you can start the codecserver application with TLS enabled using the following command:

```bash
CORS_ORIGIN=https://localhost:8080 TLS_CERT_FILE=/path/to/cert.pem TLS_KEY_FILE=/path/to/key.pem ./codecserver
```

### Waypoint

```bash
waypoint init
waypoint up
```

## Roadmap

- [ ] Test OIDC code path in the codec server: The OIDC code path in the codec server has not yet been tested and is on my to-do list for further examination and implementation.
- [ ] Create a sample for [temporalio/samples-go](https://github.com/temporalio/samples-go)

## License

This project is licensed under the [MIT License](LICENSE). By using the software or documentation in this repository, you agree to the terms of this license.
