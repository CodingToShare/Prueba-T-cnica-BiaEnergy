// Command healthcheck probes an HTTP endpoint and exits 0 only on 200 OK. It
// exists for container health checks: the runtime image (distroless) has no
// shell or curl.
//
// Usage: healthcheck [url]   (default http://127.0.0.1:8080/readyz)
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

const defaultURL = "http://127.0.0.1:8080/readyz"

func main() {
	url := defaultURL
	if len(os.Args) > 1 {
		url = os.Args[1]
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "unhealthy:", err)
		os.Exit(1)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy: HTTP", resp.StatusCode)
		os.Exit(1)
	}
}
