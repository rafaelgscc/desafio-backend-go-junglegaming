package httpadapter

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

var (
	ErrHTTPHandlerRequired  = errors.New("HTTP handler is required")
	ErrHTTPAddressRequired  = errors.New("HTTP address is required")
	ErrHTTPServerStarted    = errors.New("HTTP server already started")
	ErrHTTPServerNotStarted = errors.New("HTTP server is not started")
)

type Server struct {
	httpServer *http.Server

	mutex    sync.RWMutex
	listener net.Listener
	started  bool
	done     chan error
}

func NewServer(handler http.Handler, httpConfig config.HTTPConfig) (*Server, error) {
	if handler == nil {
		return nil, ErrHTTPHandlerRequired
	}
	if httpConfig.Address == "" {
		return nil, ErrHTTPAddressRequired
	}

	return &Server{
		httpServer: &http.Server{
			Addr:              httpConfig.Address,
			Handler:           handler,
			ReadHeaderTimeout: httpConfig.ReadHeaderTimeout,
			ReadTimeout:       httpConfig.ReadTimeout,
			WriteTimeout:      httpConfig.WriteTimeout,
			IdleTimeout:       httpConfig.IdleTimeout,
		},
		done: make(chan error, 1),
	}, nil
}

func (server *Server) Start(ctx context.Context) error {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	if server.started {
		return ErrHTTPServerStarted
	}

	listener, err := (&net.ListenConfig{}).Listen(
		ctx,
		"tcp",
		server.httpServer.Addr,
	)
	if err != nil {
		return err
	}

	server.listener = listener
	server.started = true

	go func() {
		err := server.httpServer.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		server.done <- err
		close(server.done)
	}()

	return nil
}

func (server *Server) Stop(ctx context.Context) error {
	server.mutex.RLock()
	started := server.started
	server.mutex.RUnlock()
	if !started {
		return ErrHTTPServerNotStarted
	}

	return server.httpServer.Shutdown(ctx)
}

func (server *Server) Address() string {
	server.mutex.RLock()
	defer server.mutex.RUnlock()

	if server.listener == nil {
		return ""
	}

	return server.listener.Addr().String()
}

func (server *Server) Done() <-chan error {
	return server.done
}
