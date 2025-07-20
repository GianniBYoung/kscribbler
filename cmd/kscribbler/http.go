package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"fmt"
	"net/http"
)

//go:embed certs/hardcover.pem
var hardcoverCert []byte

const apiURL = "https://api.hardcover.app/v1/graphql"

// http client with embedded CA bundle for api.hardcover.app
func newHTTPClient() (*http.Client, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(hardcoverCert) {
		return nil, fmt.Errorf("failed to parse embedded CA bundle")
	}
	tlsConfig := &tls.Config{
		RootCAs: pool,
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{Transport: transport}, nil
}

func newHardcoverRequest(ctx context.Context, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authToken)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
