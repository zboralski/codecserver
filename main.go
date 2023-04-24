package main

import (
	"crypto/tls"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strconv"

	"github.com/hashicorp/vault-client-go"
	"github.com/zboralski/codecserver/transit"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/log/tag"
)

var (
	provider *Provider
	logger   log.Logger

	port         int
	providerFlag string
	audience     string
	web          string
	debug        bool

	tlsCertFile string
	tlsKeyFile  string
)

var namespaces = []string{"default", "spread"}

func main() {
	flag.Parse()

	tlsCertFile = getFromFlagOrEnv(tlsCertFile, "TLS_CERT_FILE")
	tlsKeyFile = getFromFlagOrEnv(tlsKeyFile, "TLS_KEY_FILE")

	if (tlsCertFile == "") != (tlsKeyFile == "") {
		logger.Fatal("Both TLS cert and key must be provided if either is specified")
	}

	if portStr := os.Getenv("PORT"); portStr != "" {
		var err error
		port, err = strconv.Atoi(portStr)
		if err != nil {
			logger.Fatal("error parsing PORT environment variable", tag.NewErrorTag(err))
		}
	}

	client, err := vault.New(
		vault.WithEnvironment(),
	)
	if err != nil {
		logger.Fatal("error creating vault client", tag.NewErrorTag(err))
	}

	codecs := make(map[string][]converter.PayloadCodec)
	for _, namespace := range namespaces {
		codecs[namespace] = []converter.PayloadCodec{
			&transit.Codec{
				Client: client,
				KeyID:  namespace,
			},
			converter.NewZlibCodec(converter.ZlibCodecOptions{AlwaysEncode: true}),
		}
	}

	if providerFlag != "" {
		p, err := newProvider(providerFlag)
		if err != nil {
			logger.Fatal("error", tag.NewErrorTag(err))
		}
		logger.Info("oauth", tag.NewStringTag("issuer", p.Issuer))
		if audience != "" {
			p.Audience = audience
			logger.Info("oauth", tag.NewStringTag("audience", p.Audience))
		}
	}

	handler := newPayloadCodecNamespacesHTTPHandler(codecs)

	if web != "" {
		logger.Info("CORS enabled", tag.NewStringTag("origin", web))
		handler = newCORSHTTPHandler(web, handler)
	}

	var srv *http.Server
	errCh := make(chan error, 1)

	if tlsCertFile != "" {
		srv = &http.Server{
			Addr:      "0.0.0.0:" + strconv.Itoa(port),
			Handler:   handler,
			TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13},
		}
		go func() {
			errCh <- srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
		}()
	} else {
		srv = &http.Server{
			Addr:    "0.0.0.0:" + strconv.Itoa(port),
			Handler: handler,
		}
		go func() { errCh <- srv.ListenAndServe() }()
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	select {
	case <-sigCh:
		_ = srv.Close()
	case err := <-errCh:
		logger.Fatal("error", tag.NewErrorTag(err))
	}
}

func getFromFlagOrEnv(flagValue, envKey string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(envKey)
}

// newCORSHTTPHandler wraps a HTTP handler with CORS support
func newCORSHTTPHandler(web string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", web)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Namespace")

		if r.Method == "OPTIONS" {
			return
		}

		next.ServeHTTP(w, r)
	})
}

// newPayloadEncoderOauthHTTPHandler wraps a HTTP handler with oauth support
func newPayloadEncoderOauthHTTPHandler(provider *Provider, namespace string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if provider.Authorize(namespace, r) {
			next.ServeHTTP(w, r)
			return
		}

		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	})
}

// HTTP handler for codecs.
// This remote codec server example supports URLs like: /{namespace}/encode and /{namespace}/decode
// For example, for the default namespace you would hit /default/encode and /default/decode
// It will also accept URLs: /encode and /decode with the X-Namespace set to indicate the namespace.
// func newPayloadCodecNamespacesHTTPHandler(encoders map[string][]converter.PayloadCodec, provider *Provider) http.Handler {
func newPayloadCodecNamespacesHTTPHandler(encoders map[string][]converter.PayloadCodec) http.Handler {
	mux := http.NewServeMux()

	// Add a health check route
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	codecHandlers := make(map[string]http.Handler, len(encoders))
	for namespace, codecChain := range encoders {
		handler := converter.NewPayloadCodecHTTPHandler(codecChain...)
		if provider != nil {
			handler = newPayloadEncoderOauthHTTPHandler(provider, namespace, handler)
		}
		logger.Debug("Handling namespace", tag.WorkflowNamespace(namespace))
		mux.Handle("/"+namespace+"/", handler)

		codecHandlers[namespace] = handler
	}

	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		namespace := r.Header.Get("X-Namespace")
		if namespace != "" {
			if debug {
				logger.Debug("Got namespace", tag.WorkflowNamespace(namespace))
			}
			if handler, ok := codecHandlers[namespace]; ok {
				if debug {
					logger.Debug("Got codec handler")
				}
				handler.ServeHTTP(w, r)
				return
			}
		}
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}))

	return mux
}

func init() {
	logger = log.NewCLILogger()
	flag.IntVar(&port, "port", 8081, "Port to listen on")
	flag.StringVar(&providerFlag, "provider", "", "OIDC Provider URL. Optional: Enforces oauth authentication")
	flag.StringVar(&audience, "audience", "", "OIDC Audience. Optional")
	flag.StringVar(&web, "web", "http://localhost:8233", "Temporal Web URL. Optional: enables CORS which is required for access from Temporal Web")
	flag.StringVar(&tlsCertFile, "tls-cert-file", "", "TLS certificate file")
	flag.StringVar(&tlsKeyFile, "tls-cert-key", "", "TLS certificate key")
	flag.BoolVar(&debug, "d", false, "Debug mode")
}
