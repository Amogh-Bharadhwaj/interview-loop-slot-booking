package handlers

import (
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"

	"test/internal/testutil"
)

// newTestServer spins up the full router against a clean test DB. Lives
// inside the handlers package (rather than testutil) to avoid an import
// cycle, since testutil is shared by other packages that don't need a router.
func newTestServer(t *testing.T, holdWindow time.Duration, cacheWindowDays int) (*httptest.Server, *gorm.DB) {
	t.Helper()
	gdb := testutil.SetupDB(t)
	router := NewRouter(gdb, holdWindow, cacheWindowDays)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return srv, gdb
}
