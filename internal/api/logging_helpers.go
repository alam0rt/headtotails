package api

import (
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func requestRoute(r *http.Request) string {
	routeCtx := chi.RouteContext(r.Context())
	if routeCtx == nil {
		return "unknown"
	}
	route := routeCtx.RoutePattern()
	if route == "" {
		return "unknown"
	}
	return route
}

func requestRemoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
