package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/alam0rt/headtotails/internal/headscale"
	"github.com/alam0rt/headtotails/internal/model"
	"github.com/alam0rt/headtotails/internal/translate"
)

type usersHandler struct {
	hs headscale.HeadscaleClient
}

// ListUsers handles GET /tailnet/{tailnet}/users
func (h *usersHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.hs.ListUsers(r.Context())
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	result := make([]model.User, 0, len(users))
	for _, u := range users {
		result = append(result, translate.UserToTailscaleUser(u))
	}

	writeJSON(w, http.StatusOK, model.UserList{Users: result})
}

// GetUser handles GET /users/{userId}
func (h *usersHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	users, err := h.hs.ListUsers(r.Context())
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	u := translate.FindUserByID(users, userID)
	if u == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("user %s not found", userID))
		return
	}

	writeJSON(w, http.StatusOK, translate.UserToTailscaleUser(u))
}

// DeleteUser handles POST /users/{userId}/delete
func (h *usersHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := h.hs.DeleteUser(r.Context(), id); err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}
