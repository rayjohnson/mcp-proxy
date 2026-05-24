package menuapp

import (
	"net/http"
	"time"
)

// ServiceState represents the known state of the proxy service.
type ServiceState int

const (
	StateUnknown ServiceState = iota
	StateRunning
	StateStopped
)

// StartPoller begins polling the proxy health endpoint every 3 seconds.
// It returns a channel that delivers state updates whenever the state changes.
func StartPoller(port string) <-chan ServiceState {
	ch := make(chan ServiceState, 1)
	go func() {
		current := StateUnknown
		client := &http.Client{Timeout: time.Second}
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		// Send initial unknown state immediately.
		ch <- StateUnknown

		for range ticker.C {
			next := probe(client, port)
			if next != current {
				current = next
				ch <- current
			}
		}
	}()
	return ch
}

func probe(client *http.Client, port string) ServiceState {
	resp, err := client.Get("http://localhost:" + port + "/health")
	if err != nil {
		return StateStopped
	}
	resp.Body.Close() //nolint:errcheck
	if resp.StatusCode == http.StatusOK {
		return StateRunning
	}
	return StateStopped
}
