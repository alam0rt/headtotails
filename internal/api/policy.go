package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/alam0rt/headtotails/internal/headscale"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type policyHandler struct {
	hs headscale.HeadscaleClient
}

// GetPolicy handles GET /tailnet/{tailnet}/acl
func (h *policyHandler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := h.hs.GetPolicy(r.Context())
	if err != nil {
		// headscale 0.28 returns NotFound when no policy has been set yet.
		// Return an empty default policy so callers get a usable response.
		if status.Code(err) == codes.NotFound || status.Code(err) == codes.Unknown {
			policy = "{}"
		} else {
			writeError(w, grpcStatusToHTTP(err), err.Error())
			return
		}
	}

	// Respond according to Accept header.
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/hujson") {
		w.Header().Set("Content-Type", "application/hujson")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(policy))
		return
	}

	// Default to JSON-wrapping the policy string.
	writeJSON(w, http.StatusOK, map[string]string{"policy": policy})
}

// SetPolicy handles POST /tailnet/{tailnet}/acl
func (h *policyHandler) SetPolicy(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	policyStr := string(body)

	// If the content type is JSON (not HuJSON), we still pass through directly.
	// headscale accepts HuJSON natively.
	if err := h.hs.SetPolicy(r.Context(), policyStr); err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"policy": policyStr})
}
