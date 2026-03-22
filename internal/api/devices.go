package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/alam0rt/headtotails/internal/headscale"
	"github.com/alam0rt/headtotails/internal/model"
	"github.com/alam0rt/headtotails/internal/translate"
)

type devicesHandler struct {
	hs          headscale.HeadscaleClient
	tailnetName string
}

// ListDevices handles GET /tailnet/{tailnet}/devices
func (h *devicesHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.hs.ListNodes(r.Context(), "")
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	devices := make([]model.Device, 0, len(nodes))
	for _, n := range nodes {
		devices = append(devices, translate.NodeToDevice(n))
	}

	writeJSON(w, http.StatusOK, model.DeviceList{Devices: devices})
}

// GetDevice handles GET /device/{deviceId}
func (h *devicesHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	node, err := h.hs.GetNode(r.Context(), id)
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, translate.NodeToDevice(node))
}

// DeleteDevice handles DELETE /device/{deviceId}
func (h *devicesHandler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	if err := h.hs.DeleteNode(r.Context(), id); err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// AuthorizeDevice handles POST /device/{deviceId}/authorized
func (h *devicesHandler) AuthorizeDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	var req model.DeviceAuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// headscale uses RegisterNode for authorization; we use the node key.
	node, err := h.hs.GetNode(r.Context(), id)
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	if req.Authorized {
		user := ""
		if u := node.GetUser(); u != nil {
			user = u.GetName()
		}
		_, err = h.hs.AuthApprove(r.Context(), user, node.GetNodeKey())
		if err != nil {
			writeError(w, grpcStatusToHTTP(err), err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// ExpireDevice handles POST /device/{deviceId}/expire
func (h *devicesHandler) ExpireDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	if err := h.hs.ExpireNode(r.Context(), id); err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RenameDevice handles POST /device/{deviceId}/name
func (h *devicesHandler) RenameDevice(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	var req model.DeviceNameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	node, err := h.hs.RenameNode(r.Context(), id, req.Name)
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, translate.NodeToDevice(node))
}

// SetDeviceTags handles POST /device/{deviceId}/tags
func (h *devicesHandler) SetDeviceTags(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	var req model.DeviceTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	node, err := h.hs.SetTags(r.Context(), id, req.Tags)
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, translate.NodeToDevice(node))
}

// GetDeviceRoutes handles GET /device/{deviceId}/routes
func (h *devicesHandler) GetDeviceRoutes(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	node, err := h.hs.GetNode(r.Context(), id)
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, translate.NodeToDeviceRoutes(node))
}

// SetDeviceRoutes handles POST /device/{deviceId}/routes
func (h *devicesHandler) SetDeviceRoutes(w http.ResponseWriter, r *http.Request) {
	id, err := parseDeviceID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid device id")
		return
	}

	var req model.DeviceRoutesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	node, err := h.hs.SetApprovedRoutes(r.Context(), id, req.Routes)
	if err != nil {
		writeError(w, grpcStatusToHTTP(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, translate.NodeToDeviceRoutes(node))
}

func parseDeviceID(r *http.Request) (uint64, error) {
	raw := chi.URLParam(r, "deviceId")
	return strconv.ParseUint(raw, 10, 64)
}
