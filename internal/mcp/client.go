package mcp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	TransportStreamableHTTP = "streamable_http"
	TransportSSE            = "sse"
	TransportStdio          = "stdio"
)

// UpstreamClient wraps a connected MCP client session to an upstream server.
type UpstreamClient struct {
	Session           *mcp.ClientSession
	DetectedTransport string
}

// Connect opens a client connection to an upstream MCP server.
// It tries Streamable HTTP first, falling back to SSE if that fails.
// The detected transport is returned so the caller can cache it.
func Connect(ctx context.Context, serverURL, authHeader, cachedTransport string) (*UpstreamClient, error) {
	httpClient := &http.Client{}
	if authHeader != "" {
		httpClient.Transport = &authRoundTripper{
			base:   http.DefaultTransport,
			header: authHeader,
		}
	}

	// Use cached transport if available.
	if cachedTransport == TransportSSE {
		session, err := connectSSE(ctx, serverURL, httpClient)
		if err != nil {
			return nil, fmt.Errorf("sse connect (cached): %w", err)
		}
		return &UpstreamClient{Session: session, DetectedTransport: TransportSSE}, nil
	}

	// Try Streamable HTTP first.
	session, err := connectStreamable(ctx, serverURL, httpClient)
	if err == nil {
		return &UpstreamClient{Session: session, DetectedTransport: TransportStreamableHTTP}, nil
	}

	// Fall back to SSE.
	session, err = connectSSE(ctx, serverURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("connect upstream %s: both transports failed: %w", serverURL, err)
	}
	return &UpstreamClient{Session: session, DetectedTransport: TransportSSE}, nil
}

func connectStreamable(ctx context.Context, serverURL string, httpClient *http.Client) (*mcp.ClientSession, error) {
	transport := &mcp.StreamableClientTransport{
		Endpoint:   serverURL,
		HTTPClient: httpClient,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-proxy", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func connectSSE(ctx context.Context, serverURL string, httpClient *http.Client) (*mcp.ClientSession, error) {
	transport := &mcp.SSEClientTransport{
		Endpoint:   serverURL,
		HTTPClient: httpClient,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-proxy", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	return session, nil
}

type authRoundTripper struct {
	base   http.RoundTripper
	header string
}

func (a *authRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", a.header)
	return a.base.RoundTrip(r)
}
