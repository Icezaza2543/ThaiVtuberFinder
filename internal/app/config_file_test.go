package app

import "testing"

// The shipped production config must pass validation; a bad limit here makes
// every Railway container exit at start (healthcheck failure).
func TestShippedConfigIsValid(t *testing.T) {
	if _, err := Load("../../config/finder.json"); err != nil {
		t.Fatalf("config/finder.json invalid: %v", err)
	}
}
