package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/model"
)

// --- Topics (clusters) ---

func (s *Server) handleTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.agentStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent system not available"})
		return
	}

	clusters, err := s.agentStore.ListActiveClusters(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if clusters == nil {
		clusters = []model.TopicCluster{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": clusters})
}

func (s *Server) handleTopicByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.agentStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent system not available"})
		return
	}

	id, err := parsePathID(r.URL.Path, "/api/v1/topics/")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid topic id"})
		return
	}

	// Check for /members sub-path
	remaining := strings.TrimPrefix(r.URL.Path, "/api/v1/topics/"+strconv.FormatInt(id, 10))
	if remaining == "/members" {
		s.handleTopicMembers(w, r, id)
		return
	}

	cluster, found, err := s.agentStore.GetCluster(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "topic not found"})
		return
	}

	members, err := s.agentStore.ListClusterMembers(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if members == nil {
		members = []model.TopicClusterMember{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"cluster": cluster,
			"members": members,
		},
	})
}

func (s *Server) handleTopicMembers(w http.ResponseWriter, r *http.Request, clusterID int64) {
	members, err := s.agentStore.ListClusterMembers(r.Context(), clusterID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if members == nil {
		members = []model.TopicClusterMember{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": members})
}

// --- Briefs ---

func (s *Server) handleBriefs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.agentStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent system not available"})
		return
	}

	level := strings.TrimSpace(r.URL.Query().Get("level"))
	if level == "" {
		level = "daily"
	}
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	briefs, err := s.agentStore.ListTopicBriefs(r.Context(), level, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if briefs == nil {
		briefs = []model.TopicBrief{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": briefs})
}

func (s *Server) handleBriefByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.agentStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent system not available"})
		return
	}

	id, err := parsePathID(r.URL.Path, "/api/v1/briefs/")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid brief id"})
		return
	}

	brief, found, err := s.agentStore.GetTopicBrief(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "brief not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": brief})
}

// --- Interests ---

func (s *Server) handleInterests(w http.ResponseWriter, r *http.Request) {
	if s.agentStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent system not available"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		profiles, err := s.agentStore.ListInterestProfiles(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if profiles == nil {
			profiles = []model.InterestProfile{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": profiles})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleInterestByID(w http.ResponseWriter, r *http.Request) {
	if s.agentStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent system not available"})
		return
	}

	id, err := parsePathID(r.URL.Path, "/api/v1/interests/")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid interest id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		profile, found, err := s.agentStore.GetInterestProfile(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "interest not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": profile})

	case http.MethodPatch:
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body failed"})
			return
		}
		var req struct {
			Weight *float64 `json:"weight"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if req.Weight == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "weight is required"})
			return
		}

		profile, found, err := s.agentStore.GetInterestProfile(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "interest not found"})
			return
		}

		// Get the current embedding to pass through (weight-only update)
		embedding, err := s.agentStore.GetInterestProfileEmbedding(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if err := s.agentStore.UpdateInterestProfileEmbedding(r.Context(), id, embedding, *req.Weight, profile.SourceArticleIDs); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		profile.Weight = *req.Weight
		writeJSON(w, http.StatusOK, map[string]any{"data": profile})

	case http.MethodDelete:
		if err := s.agentStore.DeleteInterestProfile(r.Context(), id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": "deleted"})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

// --- Agent Runs ---

func (s *Server) handleAgentRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.agentStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent system not available"})
		return
	}

	agentType := r.URL.Query().Get("type")
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	runs, err := s.agentStore.ListAgentRuns(r.Context(), agentType, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if runs == nil {
		runs = []model.AgentRun{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": runs})
}

// --- Agent Trigger ---

func (s *Server) handleAgentTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	// Agent system is shelved — will be re-enabled in a future release.
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "agent system not available"})
}

// --- Helpers ---

func parsePathID(path, prefix string) (int64, error) {
	raw := strings.TrimPrefix(path, prefix)
	// Strip any trailing path segments
	if idx := strings.Index(raw, "/"); idx >= 0 {
		raw = raw[:idx]
	}
	return strconv.ParseInt(raw, 10, 64)
}
