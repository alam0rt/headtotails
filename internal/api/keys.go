package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	v1 "github.com/juanfont/headscale/gen/go/headscale/v1"
	"github.com/alam0rt/headtotails/internal/headscale"
	"github.com/alam0rt/headtotails/internal/model"
	"github.com/alam0rt/headtotails/internal/translate"
)

type keysHandler struct {
	hs          headscale.HeadscaleClient
	tailnetName string
}

// ListKeys handles GET /tailnet/{tailnet}/keys
func (h *keysHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.hs.ListPreAuthKeys(r.Context())
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	result := make([]model.Key, 0, len(keys))
	for _, k := range keys {
		result = append(result, translate.PreAuthKeyToKey(k))
	}

	writeJSON(w, http.StatusOK, model.KeyList{Keys: result})
}

// CreateKey handles POST /tailnet/{tailnet}/keys
func (h *keysHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req model.CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// headscale requires a user ID for key creation. We list users and pick
	// the first one, or use the tailnet name as a username fallback.
	users, err := h.hs.ListUsers(r.Context())
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	var userID uint64
	if len(users) > 0 {
		userID = users[0].GetId()
	}

	protoReq := translate.KeyRequestToCreatePreAuthKeyRequest(req, userID)
	key, err := h.hs.CreatePreAuthKey(r.Context(), protoReq)
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	result := translate.PreAuthKeyToKey(key)
	writeJSON(w, http.StatusOK, result)
}

// GetKey handles GET /tailnet/{tailnet}/keys/{keyId}
func (h *keysHandler) GetKey(w http.ResponseWriter, r *http.Request) {
	keyID := chi.URLParam(r, "keyId")

	keys, err := h.hs.ListPreAuthKeys(r.Context())
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	key := findKeyByID(keys, keyID)
	if key == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("key %s not found", keyID))
		return
	}

	writeJSON(w, http.StatusOK, translate.PreAuthKeyToKey(key))
}

// DeleteKey handles DELETE /tailnet/{tailnet}/keys/{keyId}
func (h *keysHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	keyID := chi.URLParam(r, "keyId")

	keys, err := h.hs.ListPreAuthKeys(r.Context())
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	key := findKeyByID(keys, keyID)
	if key == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("key %s not found", keyID))
		return
	}

	// Expire then delete.
	if err := h.hs.ExpirePreAuthKey(r.Context(), key.GetId()); err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	if err := h.hs.DeletePreAuthKey(r.Context(), key.GetId()); err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func findKeyByID(keys []*v1.PreAuthKey, id string) *v1.PreAuthKey {
	for _, k := range keys {
		if fmt.Sprintf("%d", k.GetId()) == id {
			return k
		}
	}
	return nil
}
