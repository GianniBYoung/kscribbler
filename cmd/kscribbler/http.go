package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"log"
	"net/http"
)

//go:embed certs/lets-encrypt.pem
var hardcoverCert []byte

const apiURL = "https://api.hardcover.app/v1/graphql"

// newHTTPClient with system CA bundle and embedded CA for api.hardcover.app
func newHTTPClient() *http.Client {
	// Start with the system certificate pool
	pool, err := x509.SystemCertPool()
	if err != nil {
		// If system pool is unavailable, create a new pool
		log.Printf("Warning: Unable to load system certificate pool: %v", err)
		pool = x509.NewCertPool()
	}

	// Append the embedded Let's Encrypt certificate as a fallback
	if !pool.AppendCertsFromPEM(hardcoverCert) {
		log.Printf("Warning: Failed to parse embedded CA bundle")
	}

	tlsConfig := &tls.Config{
		RootCAs: pool,
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	return &http.Client{Transport: transport}
}

// newHardcoverRequest creates a new HTTP request to the Hardcover API with the appropriate headers.
func newHardcoverRequest(ctx context.Context, body []byte) *http.Request {
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		log.Fatalf("Failed to create newHardcoverRequest: %v", err)
	}

	req.Header.Set("Authorization", authToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(
		"User-Agent",
		"kscribbler - https://github.com/GianniBYoung/kscribbler",
	)

	return req
}

func verifyHardcoverConnection(client *http.Client, ctx context.Context) {
	req := newHardcoverRequest(ctx, []byte(`{"query": "conenctiontest  e { id }}"}`))

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to connect to Hardcover API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Hardcover API returned non-200 status: %d %s", resp.StatusCode, resp.Status)
	}
}
