package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/je4/indexer/pkg/indexer"
	"github.com/phayes/freeport"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"
)

var addr string
var workingDirectory string

func TestMain( m *testing.M) {
	var err error
	workingDirectory, err = os.Getwd()
	if err != nil {
		log.Fatalf("error getting working directory: %v", err)
	}

	// create logger instance
	log, lf := indexer.CreateLogger("indexer", "", "DEBUG")
	defer lf.Close()

	var accesslog io.Writer
	accesslog = os.Stdout

	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatalf("cannot find free port: %v", err)
	}
	addr = fmt.Sprintf("localhost:%v", port)

	srv, err := indexer.NewServer(
		10*time.Second,
		1024,
		".*/.*",
		1024,
		"swordfish",
		[]string{"HS386"},
		log,
		accesslog,
		fmt.Sprintf("%s/../../web/template/error.gohtml", workingDirectory),
		workingDirectory,
	)
	if err != nil {
		log.Panicf("cannot initialize server: %v", err)
		return
	}
	var shutdown = false
	go func() {
		if err := srv.ListenAndServe(addr, "", ""); err != nil {
			if !shutdown {
				log.Errorf("server died: %+v", err)
			}
		}
		fmt.Println("server stopped")
	}()

	exitCode := m.Run()

	shutdown = true
	srv.Shutdown(context.Background())
	os.Exit(exitCode)
}

func TestIndexer(t *testing.T) {
	requestBody, err := json.Marshal(map[string]string{
		"url": fmt.Sprintf("file:///%s/../../web/static/memoriav_logo_400x400.jpg", workingDirectory),
	})
	if err != nil {
		t.Error("cannot marshal request body")
		return
	}
	resp, err := http.Post(fmt.Sprintf("http://%s", addr ), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Errorf("error querying %s: %v", addr, err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		t.Errorf("cannot decode json result: %v", err)
		return
	}
	_mimetype, ok := result["mimetype"]
	if !ok {
		t.Errorf("no mimetype in result %v", result)
		return
	}
	mimetype, ok := _mimetype.(string)
	if !ok {
		t.Errorf("mimetype not a string: %v", mimetype)
		return
	}
	if mimetype != "image/jpeg" {
		t.Errorf("invalid mimetype: %s", mimetype)
		return
	}
}