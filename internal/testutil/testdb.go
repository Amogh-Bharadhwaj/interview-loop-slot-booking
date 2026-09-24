// Package testutil provides shared fixtures for integration tests that hit
// a real Postgres database. It is only ever imported from _test.go files.
//
// `go test ./...` runs each package as its own OS process and, by default,
// schedules several of those processes concurrently. Any package that needs
// a database therefore gets its own throwaway one — created fresh in
// TestMain and dropped when that package's tests finish — rather than
// sharing a single database across packages, which would let one package's
// TRUNCATE or sequence reset race another package's fixtures mid-test.
//
// A package that calls SetupDB must add:
//
//	func TestMain(m *testing.M) { os.Exit(testutil.RunMain(m)) }
//
// Point TEST_POSTGRES_BASE_DSN at a different Postgres instance if needed
// (default: the same local instance the app's .env already uses). Only the
// `postgres` maintenance database needs to exist ahead of time.
//
// Run the suite:
//
//	go test ./...
//
// Run with coverage (combined across all internal packages, excluding this
// test-helper package itself):
//
//	go test ./internal/config/... ./internal/handlers/... ./internal/jobs/... ./internal/models/... \
//	  -coverpkg=./internal/config/...,./internal/db/...,./internal/handlers/...,./internal/jobs/...,./internal/models/... \
//	  -coverprofile=coverage.out
//	go tool cover -func=coverage.out | tail -1
//
// Last measured total: 84.0% of statements. The two functions sitting at
// 0% (jobs.RunReaper, jobs.RunCacheRefresh) are the ticker-driven infinite
// loops that call the (fully tested) sweep/refresh functions on a timer —
// not practically worth driving through a real ticker interval in a test.
package testutil

import (
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	appdb "test/internal/db"
)

func baseDSN() string {
	if v := os.Getenv("TEST_POSTGRES_BASE_DSN"); v != "" {
		return v
	}
	return "host=localhost user=postgres password=postgres port=5432 sslmode=disable"
}

func dsnFor(dbName string) string {
	return fmt.Sprintf("%s dbname=%s", baseDSN(), dbName)
}

var sharedDB *gorm.DB

// RunMain creates a uniquely-named database for this test binary, migrates
// it, runs the package's tests, then drops the database — so a package's
// tests are fully isolated from every other package's. Returns the process
// exit code; call as `os.Exit(testutil.RunMain(m))` from a TestMain.
func RunMain(m *testing.M) (code int) {
	dbName := fmt.Sprintf("interviewdb_test_%d", os.Getpid())

	admin, err := gorm.Open(postgres.Open(dsnFor("postgres")), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		fmt.Fprintf(os.Stderr, "testutil: connect to admin db: %v\n", err)
		return 1
	}
	if err := admin.Exec(fmt.Sprintf(`CREATE DATABASE %q`, dbName)).Error; err != nil {
		fmt.Fprintf(os.Stderr, "testutil: create test db %s: %v\n", dbName, err)
		return 1
	}
	defer func() {
		if err := admin.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, dbName)).Error; err != nil {
			fmt.Fprintf(os.Stderr, "testutil: drop test db %s: %v\n", dbName, err)
		}
	}()

	gdb, err := appdb.Connect(dsnFor(dbName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "testutil: connect to test db %s: %v\n", dbName, err)
		return 1
	}
	if err := appdb.Migrate(gdb); err != nil {
		fmt.Fprintf(os.Stderr, "testutil: migrate test db %s: %v\n", dbName, err)
		return 1
	}
	sharedDB = gdb

	code = m.Run()

	if sqlDB, err := gdb.DB(); err == nil {
		sqlDB.Close()
	}
	return code
}

// SetupDB truncates every app table before returning the package's shared
// test connection, and registers a cleanup to truncate again afterward —
// so tests are independent of run order within the package. The connection
// itself, and its eventual drop, are managed by RunMain.
func SetupDB(t *testing.T) *gorm.DB {
	t.Helper()
	if sharedDB == nil {
		t.Fatal("testutil.SetupDB: no test database set up — this package's TestMain must call testutil.RunMain(m)")
	}
	truncateAll(t, sharedDB)
	t.Cleanup(func() { truncateAll(t, sharedDB) })
	return sharedDB
}

func truncateAll(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	err := gdb.Exec(`TRUNCATE TABLE slot_participants, slots, meeting_rooms, offices, users RESTART IDENTITY CASCADE`).Error
	if err != nil {
		t.Fatalf("truncate test db: %v", err)
	}
}
