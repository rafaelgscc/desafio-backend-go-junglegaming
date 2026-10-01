package httpadapter

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

func TestServerStartsServesAndStops(t *testing.T) {
	handler := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	server, err := NewServer(handler, config.HTTPConfig{
		Address:           "127.0.0.1:0",
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
		IdleTimeout:       5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}

	startContext, cancelStart := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelStart()
	if err := server.Start(startContext); err != nil {
		t.Fatalf("Start() unexpected error: %v", err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + server.Address() + "/test")
	if err != nil {
		t.Fatalf("GET running server unexpected error: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	stopContext, cancelStop := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelStop()
	if err := server.Stop(stopContext); err != nil {
		t.Fatalf("Stop() unexpected error: %v", err)
	}

	select {
	case err := <-server.Done():
		if err != nil {
			t.Fatalf("server completion error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop within timeout")
	}
}
