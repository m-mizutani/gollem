// This example calls Parallel Search MCP through gollem's ToolSet interface.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gollem-dev/gollem/mcp"
	"github.com/m-mizutani/goerr/v2"
)

const userAgent = "gollem-parallel-search-example/1.0"

// identifiedTransport adds the project identity to every MCP request.
type identifiedTransport struct {
	base http.RoundTripper
}

func (t identifiedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("User-Agent", userAgent)
	return t.base.RoundTrip(clone)
}

func main() {
	objective := flag.String("objective", "Find the official Go release notes", "Information to find")
	query := flag.String("query", "Go official release notes", "Keyword search query")
	url := flag.String("url", "", "Fetch this URL instead of searching")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	if err := run(ctx, *objective, *query, *url); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, objective, query, url string) (err error) {
	// No Authorization header or saved credentials are used. The custom client
	// also bounds individual HTTP requests and identifies discovery and calls.
	client, err := mcp.NewStreamableHTTP(ctx, "https://search.parallel.ai/mcp",
		mcp.WithStreamableHTTPClientInfo("gollem-parallel-search-example", "1.0"),
		mcp.WithStreamableHTTPClient(&http.Client{
			Timeout:   60 * time.Second,
			Transport: identifiedTransport{base: http.DefaultTransport},
		}))
	if err != nil {
		return goerr.Wrap(err, "connect to Parallel Search MCP")
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			if err == nil {
				err = goerr.Wrap(closeErr, "close Parallel Search MCP")
			} else {
				fmt.Fprintln(os.Stderr, "close Parallel Search MCP:", closeErr)
			}
		}
	}()

	// Discover tools through the same ToolSet interface used by WithToolSets.
	specs, err := client.Specs(ctx)
	if err != nil {
		return goerr.Wrap(err, "discover Parallel tools")
	}

	name := "web_search"
	args := map[string]any{
		"objective":      objective,
		"search_queries": []string{query},
	}
	if url != "" {
		name = "web_fetch"
		args = map[string]any{"urls": []string{url}}
	}

	for _, spec := range specs {
		if spec.Name != name {
			continue
		}
		result, err := client.Run(ctx, name, args)
		if err != nil {
			return goerr.Wrap(err, "execute Parallel tool", goerr.V("tool", name))
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			return goerr.Wrap(err, "write tool output")
		}
		return nil
	}
	return goerr.New("Parallel tool is unavailable", goerr.V("tool", name))
}
