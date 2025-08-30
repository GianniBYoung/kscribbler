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

//go:embed certs/hardcover.pem
var hardcoverCert []byte

const apiURL = "https://api.hardcover.app/v1/graphql"

// newHTTPClient with embedded CA bundle for api.hardcover.app
func newHTTPClient() *http.Client {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(hardcoverCert) {
		log.Fatalf("failed to parse embedded CA bundle... exiting")
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
