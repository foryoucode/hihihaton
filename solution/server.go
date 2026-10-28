package solution

import (
	"errors"
	"log/slog"
	"net/http"
)

type Server struct {
	address string
	logger  *slog.Logger
	handler http.Handler
}

func NewServer(address string, logger *slog.Logger, handler http.Handler) *Server {
	return &Server{
		address: address,
		logger:  logger,
		handler: handler,
	}
}

func (s *Server) Start() error {
	s.logger.Info("server has been started", "address", s.address)

	err := http.ListenAndServe(s.address, s.handler)
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
