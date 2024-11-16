package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"github.com/stretchr/testify/assert"
)

type Table struct {
	Name string `json:"name"`
}

func TestAgent(t *testing.T) {
	ch := make(chan int)

	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	srv := &http.Server{Addr: ":3333"}

	http.HandleFunc("/testorg/sync/postgresql", func (w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.Header.Get("Content-Type"), "application/json", "Got a JSON request")
		dec := json.NewDecoder(r.Body)
		var dbs []struct{
			Database string `json:"database"`
			Schema string `json:"schema"`
			Tables []Table
		}
		err := dec.Decode(&dbs)
		assert.Equal(t, nil, err, "Could decode JSON")
		assert.Equal(t, strings.Split(os.Getenv("COMPREHEND_DB_CONN_INFO"), "/")[3], dbs[0].Database, "Found test database")
		assert.Equal(t, "public", dbs[0].Schema, "Found public schema")
		idx := slices.IndexFunc(dbs[0].Tables, func(t Table) bool { return t.Name == "organizations" })
		assert.GreaterOrEqual(t, idx, 0, "Table organizations found")
		w.WriteHeader(204)
		io.WriteString(w, "OK")
		ch <- 1
	})

	go func() {
		defer httpServerExitDone.Done()
		err := srv.ListenAndServe()
		assert.Equal(t, err, http.ErrServerClosed, "server closed successfully")
	}()

	cmd := exec.Command("./agent",
		"--comprehend-url", "http://localhost:3333/",
		"--organization", "testorg",
		"--document", "arch",
		os.Getenv("COMPREHEND_DB_CONN_INFO") + "?sslmode=disable")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGCHLD)
	go func() {
		_ = <-sigc
		state, err := cmd.Process.Wait()
		assert.Equal(t, err, nil, "Waited successfully for child")
		assert.Equal(t, 0, state.ExitCode(), "Child exited successfully")
		ch <- 1
	}()

	err := cmd.Start()
	assert.Equal(t, err, nil, "agent started successfully")

	_ = <-ch
	srv.Shutdown(context.TODO())

	err = cmd.Process.Kill()

	httpServerExitDone.Wait()
}
