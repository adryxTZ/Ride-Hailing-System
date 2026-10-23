// Command probe is the container healthcheck for the distroless image, which has
// no shell, curl or wget. It GETs one URL and exits 0 on a 2xx answer, 1 otherwise.
//
//	probe                                  checks http://127.0.0.1:8080/v1/ready
//	probe http://127.0.0.1:8080/v1/health  checks the given URL
//
// Docker and compose run it as: healthcheck: ["CMD", "/app/probe"].
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// defaultURL is the readiness endpoint of the API inside its own container.
const defaultURL = "http://127.0.0.1:8080/v1/ready"

// timeout keeps a hung server from stalling the healthcheck past Docker's own timeout.
const timeout = 3 * time.Second

func main() {
	url := defaultURL
	if len(os.Args) > 1 {
		url = os.Args[1]
	}
	if err := check(url); err != nil {
		fmt.Fprintln(os.Stderr, "probe:", err)
		os.Exit(1)
	}
}

// check returns nil when url answers with a 2xx status within the timeout.
func check(url string) error {
	client := http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s answered %d", url, resp.StatusCode)
	}
	return nil
}
