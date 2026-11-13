package bot

import (
	"fmt"
	"time"
)

const readyPollInterval = 100 * time.Millisecond

// openUntilReady opens a connection and waits for ready, retrying with a fresh
// connection when it does not get ready in time. dave-go sometimes never
// completes the MLS join on the first try, leaving the bot silent in the channel.
func openUntilReady[C any](attempts int, timeout time.Duration, open func() (C, error), ready func(C) bool, discard func(C)) (C, error) {
	var zero C
	for attempt := 1; attempt <= attempts; attempt++ {
		conn, err := open()
		if err != nil {
			return zero, err
		}
		if waitUntil(timeout, func() bool { return ready(conn) }) {
			return conn, nil
		}
		discard(conn)
	}
	return zero, fmt.Errorf("connection not ready after %d attempts", attempts)
}

func waitUntil(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for !condition() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(min(readyPollInterval, timeout))
	}
	return true
}
