package server

import (
	"net/http"
	"time"
	"wheres-my-pizza/internal/core/domain/types"
)

type responseWriterWrapper struct {
	http.ResponseWriter
	status int
}

func (a *API) Middleware(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		rw := &responseWriterWrapper{ResponseWriter: w}

		a.log.Debug(
			r.Context(),
			types.ActionOrderReceived,
			"started",
			"method", r.Method,
			"URL", r.URL.Path,
			"host", r.Host,
		)

		next.ServeHTTP(rw, r)

		duration := time.Since(startTime)

		a.log.Debug(
			r.Context(),
			types.ActionOrderReceived,
			"completed",
			"method", r.Method,
			"URL", r.URL.Path,
			"status", rw.status,
			"duration", duration,
		)
	})
}
