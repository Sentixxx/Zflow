package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type updateNetworkSettingsRequest struct {
	ProxyURL string `json:"proxy_url"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleNetworkSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		value, ok, err := s.store.GetSetting(settingKeyNetworkProxy)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load network settings"})
			return
		}
		if !ok {
			value = ""
		}
		writeJSON(w, http.StatusOK, map[string]string{"proxy_url": strings.TrimSpace(value)})
	case http.MethodPatch:
		defer r.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		var req updateNetworkSettingsRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		proxyURL := strings.TrimSpace(req.ProxyURL)
		if err := validateProxyURL(proxyURL); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := s.store.SetSetting(settingKeyNetworkProxy, proxyURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save network settings"})
			return
		}
		if err := s.applyNetworkProxy(proxyURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to apply proxy setting"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"proxy_url": proxyURL})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
