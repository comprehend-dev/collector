package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/comprehend-dev/collector/collectors"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureTable = "collector_test_widgets"

type collectedColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

type collectedKey struct {
	ColumnNames []string `json:"column_names"`
}

type collectedTable struct {
	Name       string            `json:"name"`
	Columns    []collectedColumn `json:"columns"`
	Rows       int               `json:"rows"`
	PrimaryKey *collectedKey     `json:"primary_key"`
}

type collectedDatabase struct {
	Database string           `json:"database"`
	Schema   string           `json:"schema"`
	Tables   []collectedTable `json:"tables"`
}

type databasePayload struct {
	Host      string              `json:"host"`
	Port      int                 `json:"port"`
	Databases []collectedDatabase `json:"databases"`
}

// createFixture gives the collector something of our own making to find, so that the test does not
// depend on whatever else happens to live in the database it is pointed at.
func createFixture(t *testing.T, connStr string) {
	t.Helper()

	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err, "opening the test database")
	t.Cleanup(func() {
		_, _ = db.Exec("DROP TABLE IF EXISTS " + fixtureTable)
		db.Close()
	})
	require.NoError(t, db.Ping(), "connecting to the test database")

	_, err = db.Exec("DROP TABLE IF EXISTS " + fixtureTable)
	require.NoError(t, err, "dropping a leftover fixture table")
	_, err = db.Exec("CREATE TABLE " + fixtureTable +
		" (id serial PRIMARY KEY, name text NOT NULL, description text)")
	require.NoError(t, err, "creating the fixture table")
	_, err = db.Exec("INSERT INTO " + fixtureTable +
		" (name, description) VALUES ('first', 'a widget'), ('second', NULL)")
	require.NoError(t, err, "filling the fixture table")
}

func buildCollector(t *testing.T, version string) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "collector")
	build := exec.Command("go", "build", "-ldflags", "-X main.version="+version, "-o", binary, ".")
	build.Stderr = os.Stderr
	require.NoError(t, build.Run(), "building the collector")
	return binary
}

// Releases stamp the version in through ldflags, and the collector reports it to ingestion, so a
// build that silently loses it would be hard to notice.
func TestVersionFlag(t *testing.T) {
	binary := buildCollector(t, "1.2.3-test")

	out, err := exec.Command(binary, "--version").Output()
	require.NoError(t, err, "running the collector with --version")
	assert.Equal(t, "comprehend.dev collector 1.2.3-test\n", string(out))
}

func TestPostgresCollector(t *testing.T) {
	connStr := os.Getenv("COLLECTOR_TEST_POSTGRES")
	if connStr == "" {
		t.Skip("set COLLECTOR_TEST_POSTGRES to a PostgreSQL connection string to run this test")
	}

	createFixture(t, connStr)
	binary := buildCollector(t, "dev")

	payloads := make(chan databasePayload, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/testorg/sync/postgresql", r.URL.Path, "ingested to the collector's own route")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var payload databasePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("could not decode the ingested payload: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)

		select {
		case payloads <- payload:
		default:
		}
	}))
	defer server.Close()

	cmd := exec.Command(binary,
		"--comprehend-url", server.URL+"/",
		"--organization", "testorg",
		"--apikey", "test-key",
		"--postgresql", connStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start(), "starting the collector")

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	defer func() {
		_ = cmd.Process.Kill()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
		}
	}()

	var payload databasePayload
	select {
	case payload = <-payloads:
	case err := <-exited:
		t.Fatalf("the collector exited before ingesting anything: %s", err)
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for the collector to ingest")
	}

	assert.NotEmpty(t, payload.Host, "reported the database host")

	var fixture *collectedTable
	for i := range payload.Databases {
		database := &payload.Databases[i]
		if database.Schema != "public" {
			continue
		}
		for j := range database.Tables {
			if database.Tables[j].Name == fixtureTable {
				fixture = &database.Tables[j]
			}
		}
	}
	require.NotNil(t, fixture, "collected the fixture table from the public schema")

	assert.Equal(t, 2, fixture.Rows, "counted the fixture rows")

	require.NotNil(t, fixture.PrimaryKey, "collected the primary key")
	assert.Equal(t, []string{"id"}, fixture.PrimaryKey.ColumnNames)

	columns := map[string]collectedColumn{}
	for _, column := range fixture.Columns {
		columns[column.Name] = column
	}
	assert.Equal(t, collectedColumn{Name: "id", Type: "int4", Nullable: false}, columns["id"])
	assert.Equal(t, collectedColumn{Name: "name", Type: "text", Nullable: false}, columns["name"])
	assert.Equal(t, collectedColumn{Name: "description", Type: "text", Nullable: true}, columns["description"])
}

// usage runs the collector with --help and returns what it printed. Go's flag package writes the
// usage to stderr and exits non-zero, so neither is a failure here.
func usage(t *testing.T, binary string) string {
	t.Helper()

	cmd := exec.Command(binary, "--help")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run()
	return out.String()
}

// The collectors used to be listed by ranging over the map they register themselves in, so the
// usage came out in a different order on every run and could not be documented.
func TestUsageListsCollectorsInAStableOrder(t *testing.T) {
	binary := buildCollector(t, "dev")

	first := usage(t, binary)
	for run := 0; run < 5; run++ {
		assert.Equal(t, first, usage(t, binary), "the usage changed between runs")
	}

	for _, section := range []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"Options", regexp.MustCompile(`(?m)^ +--([a-z0-9]+) <value>`)},
		{"Arguments", regexp.MustCompile(`(?m)^ +([a-z0-9]+)://\.\.\.`)},
	} {
		var listed []string
		for _, match := range section.pattern.FindAllStringSubmatch(first, -1) {
			if _, isCollector := collectors.Collectors[match[1]]; isCollector {
				listed = append(listed, match[1])
			}
		}

		assert.Equal(t, collectors.Names(), listed,
			"%s does not list every collector in sorted order", section.name)
	}
}

// The descriptions line up in a column, and the README reproduces the usage, so a new option that
// does not fit would quietly leave both crooked.
func TestUsageDescriptionsLineUp(t *testing.T) {
	usage := usage(t, buildCollector(t, "dev"))

	entry := regexp.MustCompile(`(?m)^ {4}(\S.*?) {2,}(\S.*)$`)
	matches := entry.FindAllStringSubmatchIndex(usage, -1)
	require.NotEmpty(t, matches, "the usage lists no options at all")

	for _, match := range matches {
		line, column := usage[match[0]:match[1]], match[4]-match[0]
		assert.Equal(t, 28, column, "description is not in the usual column: %q", line)
	}
}
