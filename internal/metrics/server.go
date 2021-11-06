package metrics

import (
	"context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

type Server struct {
	server http.Server
}

func NewServer(addr string) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/.well-known/ready", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
	})

	return &Server{
		server: http.Server{
			Handler: mux,
			Addr:    addr,
		},
	}
}

func (s *Server) ListenAndServe() error {
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
