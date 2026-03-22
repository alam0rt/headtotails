package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/alam0rt/headtotails/internal/model"
	"github.com/go-chi/chi/v5/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, statusCode int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(model.Error{Message: msg})
}

// writeJSON writes a JSON success response.
func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(v)
}

// notImplementedReason returns a handler that responds with 501 and a specific reason.
func notImplementedReason(reason string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotImplemented, reason)
	}
}

// grpcStatusToHTTP maps gRPC status codes to HTTP status codes.
func grpcStatusToHTTP(err error) int {
	switch status.Code(err) {
	case codes.NotFound:
		return http.StatusNotFound
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.InvalidArgument:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeGRPCError(w http.ResponseWriter, r *http.Request, err error) {
	grpcCode := status.Code(err)
	httpStatus := grpcStatusToHTTP(err)
	observeGRPCError(grpcCode)

	slog.Error("upstream headscale error",
		"method", r.Method,
		"path", r.URL.Path,
		"status", httpStatus,
		"grpc_code", grpcCode.String(),
		"request_id", middleware.GetReqID(r.Context()),
		"error", err.Error(),
	)
	writeError(w, httpStatus, err.Error())
}
