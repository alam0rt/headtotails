package api

import (
	"context"
	"encoding/json"
	"errors"
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
	user, err := h.resolveTailnetUser(r.Context(), chi.URLParam(r, "tailnet"))
	if err != nil {
		writeResolveError(w, r, err)
		return
	}

	keys, err := h.hs.ListPreAuthKeys(r.Context(), user.GetName())
	if err != nil {
		writeGRPCError(w, r, err)
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
	user, err := h.resolveTailnetUser(r.Context(), chi.URLParam(r, "tailnet"))
	if err != nil {
		writeResolveError(w, r, err)
		return
	}

	var req model.CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	protoReq := translate.KeyRequestToCreatePreAuthKeyRequest(req, user.GetId())
	key, err := h.hs.CreatePreAuthKey(r.Context(), protoReq)
	if err != nil {
		writeGRPCError(w, r, err)
		return
	}

	result := translate.PreAuthKeyToKey(key)
	writeJSON(w, http.StatusOK, result)
}

// GetKey handles GET /tailnet/{tailnet}/keys/{keyId}
func (h *keysHandler) GetKey(w http.ResponseWriter, r *http.Request) {
	keyID := chi.URLParam(r, "keyId")

	user, err := h.resolveTailnetUser(r.Context(), chi.URLParam(r, "tailnet"))
	if err != nil {
		writeResolveError(w, r, err)
		return
	}

	keys, err := h.hs.ListPreAuthKeys(r.Context(), user.GetName())
	if err != nil {
		writeGRPCError(w, r, err)
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

	user, err := h.resolveTailnetUser(r.Context(), chi.URLParam(r, "tailnet"))
	if err != nil {
		writeResolveError(w, r, err)
		return
	}

	keys, err := h.hs.ListPreAuthKeys(r.Context(), user.GetName())
	if err != nil {
		writeGRPCError(w, r, err)
		return
	}

	key := findKeyByID(keys, keyID)
	if key == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("key %s not found", keyID))
		return
	}

	// Expire then delete.
	if err := h.hs.ExpirePreAuthKey(r.Context(), key.GetId()); err != nil {
		writeGRPCError(w, r, err)
		return
	}

	if err := h.hs.DeletePreAuthKey(r.Context(), key.GetId()); err != nil {
		writeGRPCError(w, r, err)
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

type keyResolveError struct {
	status int
	msg    string
}

func (e *keyResolveError) Error() string { return e.msg }

func writeResolveError(w http.ResponseWriter, r *http.Request, err error) {
	var e *keyResolveError
	if errors.As(err, &e) {
		writeError(w, e.status, e.msg)
		return
	}
	writeGRPCError(w, r, err)
}

func (h *keysHandler) resolveTailnetUser(ctx context.Context, tailnet string) (*v1.User, error) {
	users, err := h.hs.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, &keyResolveError{
			status: http.StatusNotFound,
			msg:    "no users exist in headscale",
		}
	}

	resolved := tailnet
	if resolved == "-" {
		resolved = h.tailnetName
	}

	if resolved == "-" {
		if len(users) != 1 {
			return nil, &keyResolveError{
				status: http.StatusBadRequest,
				msg:    "tailnet is ambiguous; set TAILNET_NAME to a dedicated user",
			}
		}
		return users[0], nil
	}

	if h.tailnetName != "-" && tailnet != "-" && tailnet != h.tailnetName {
		return nil, &keyResolveError{
			status: http.StatusNotFound,
			msg:    fmt.Sprintf("tailnet %s not found", tailnet),
		}
	}

	for _, user := range users {
		if user.GetName() == resolved {
			return user, nil
		}
	}

	return nil, &keyResolveError{
		status: http.StatusNotFound,
		msg:    fmt.Sprintf("tailnet %s not found", tailnet),
	}
}
