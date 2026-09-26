package app

import (
	"os"
	"testing"
)

// The shipped production config must pass validation; a bad limit here makes
// every Railway container exit at start (healthcheck failure).
func TestShippedConfigIsValid(t *testing.T) {
	const path = "../../config/finder.json"
	if _, err := os.Stat(path); err != nil {
		t.Skip("config not present (Docker build copies only cmd/ and internal/)")
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("config/finder.json invalid: %v", err)
	}
}
