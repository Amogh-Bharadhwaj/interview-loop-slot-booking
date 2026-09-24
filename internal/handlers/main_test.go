package handlers

import (
	"os"
	"testing"

	"test/internal/testutil"
)

// TestMain gives this package's tests their own throwaway Postgres
// database (created and dropped by testutil.RunMain) instead of sharing
// one with other packages' test binaries.
func TestMain(m *testing.M) {
	os.Exit(testutil.RunMain(m))
}
