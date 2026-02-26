package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/model"
)

func (s *Server) handleAISettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := s.loadAISettings()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load ai settings"})
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodPatch:
		defer r.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		var req updateAISettingsRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		cfg := aiSettings{APIKey: strings.TrimSpace(req.APIKey), BaseURL: strings.TrimSpace(req.BaseURL), Model: strings.TrimSpace(req.Model), TargetLang: strings.TrimSpace(req.TargetLang)}
		if cfg.BaseURL != "" {
			parsed, err := url.Parse(cfg.BaseURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "base_url is invalid"})
				return
			}
		}
		if cfg.TargetLang == "" {
			cfg.TargetLang = defaultAITargetLang
		}

		if err := s.store.SetSetting(settingKeyAIApiKey, cfg.APIKey); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save ai settings"})
			return
		}
		if err := s.store.SetSetting(settingKeyAIBaseURL, cfg.BaseURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save ai settings"})
			return
		}
		if err := s.store.SetSetting(settingKeyAIModel, cfg.Model); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save ai settings"})
			return
		}
		if err := s.store.SetSetting(settingKeyAITargetLang, cfg.TargetLang); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save ai settings"})
			return
		}

		writeJSON(w, http.StatusOK, cfg)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func extractArticleTranslationParagraphs(article model.Article) []string {
	candidates := []string{article.FullContent, article.Summary, article.Title}
	for _, raw := range candidates {
		text := strings.TrimSpace(raw)
		if text == "" {
			continue
		}
		if looksLikePDFPayload(text) {
			continue
		}
		paragraphs := splitTranslationParagraphs(text)
		if len(paragraphs) > 0 {
			return paragraphs
		}
	}
	return nil
}

func looksLikePDFPayload(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(strings.ToUpper(trimmed), "%PDF-") {
		return true
	}
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "xref") && strings.Contains(lower, "endobj")
}

func splitTranslationParagraphs(raw string) []string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}

	if strings.Contains(text, "<") && strings.Contains(text, ">") {
		blockBreaks := regexp.MustCompile(`(?is)</(p|div|li|h[1-6]|blockquote|section|article|pre|tr|ul|ol)>|<br\s*/?>`)
		text = blockBreaks.ReplaceAllString(text, "\n\n")
		tags := regexp.MustCompile(`(?s)<[^>]*>`)
		text = tags.ReplaceAllString(text, " ")
		text = html.UnescapeString(text)
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	parts := regexp.MustCompile(`\n\s*\n+`).Split(text, -1)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		normalized := strings.TrimSpace(p)
		if normalized == "" {
			continue
		}
		normalized = strings.Join(strings.Fields(normalized), " ")
		if normalized != "" {
			result = append(result, normalized)
		}
	}

	if len(result) == 0 {
		fallback := strings.TrimSpace(strings.Join(strings.Fields(text), " "))
		if fallback != "" {
			return []string{fallback}
		}
	}
	return result
}

func (s *Server) streamArticleTranslation(w http.ResponseWriter, r *http.Request, articleID int64) {
	article, ok := s.store.GetArticle(articleID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	settings, err := s.loadAISettings()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load ai settings"})
		return
	}
	req := translateArticleRequest{TargetLang: settings.TargetLang}
	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
	}
	targetLang := strings.TrimSpace(req.TargetLang)
	if targetLang == "" {
		targetLang = settings.TargetLang
	}
	if targetLang == "" {
		targetLang = defaultAITargetLang
	}

	paragraphs := extractArticleTranslationParagraphs(article)
	if len(paragraphs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "article content is empty"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming is not supported"})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writer := bufio.NewWriter(w)
	emit := func(event translateStreamEvent) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := writer.Write(payload); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := emit(translateStreamEvent{Type: "start", ArticleID: articleID, TargetLang: targetLang, Total: len(paragraphs), Sources: paragraphs}); err != nil {
		return
	}

	err = s.translateParagraphs(r.Context(), paragraphs, targetLang, settings, func(index int, total int, source string, translated string) error {
		return emit(translateStreamEvent{Type: "chunk", ArticleID: articleID, TargetLang: targetLang, Total: total, Index: index, Source: source, Translated: translated})
	})
	if err != nil {
		_ = emit(translateStreamEvent{Type: "error", ArticleID: articleID, Error: err.Error()})
		return
	}
	_ = emit(translateStreamEvent{Type: "done", ArticleID: articleID, Total: len(paragraphs)})
}

func (s *Server) translateTextWithAI(ctx context.Context, text string, targetLang string, settings aiSettings, history []translationPair) (string, error) {
	apiKey := strings.TrimSpace(settings.APIKey)
	if apiKey == "" {
		return "", errors.New("missing AI API key")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(firstNonEmpty(settings.BaseURL, defaultAIBaseURL)), "/")
	model := strings.TrimSpace(firstNonEmpty(settings.Model, defaultAIModel))
	contextText := buildTranslationContext(history)

	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqPayload := map[string]any{
		"model": model,
		"messages": []chatMessage{
			{Role: "system", Content: "You are a precise translator. Keep terminology consistent with prior translated context. Return only translated plain text for the current segment without explanations."},
			{Role: "user", Content: fmt.Sprintf(
				"Translate the CURRENT segment to %s.\nRequirements:\n1) Preserve meaning accurately.\n2) Keep names/terms consistent with prior context.\n3) Output only translated text for CURRENT segment.\n\nPrior translated context (for consistency):\n%s\n\nCURRENT segment:\n%s",
				targetLang,
				contextText,
				text,
			)},
		},
		"temperature": 0.2,
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClientForAI().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("empty translation response")
	}
	result := strings.TrimSpace(out.Choices[0].Message.Content)
	if result == "" {
		return "", errors.New("empty translated content")
	}
	return result, nil
}

func (s *Server) translateParagraphs(
	ctx context.Context,
	paragraphs []string,
	targetLang string,
	settings aiSettings,
	onChunk func(index int, total int, source string, translated string) error,
) error {
	total := len(paragraphs)
	history := make([]translationPair, 0, total)
	for idx, source := range paragraphs {
		translated, err := s.translateTextWithAI(ctx, source, targetLang, settings, history)
		if err != nil {
			return err
		}
		if err := onChunk(idx+1, total, source, translated); err != nil {
			return err
		}
		history = append(history, translationPair{Source: source, Translated: translated})
	}
	return nil
}

func buildTranslationContext(history []translationPair) string {
	if len(history) == 0 {
		return "(none)"
	}
	const (
		maxPairs = 4
		maxChars = 2200
	)
	start := 0
	if len(history) > maxPairs {
		start = len(history) - maxPairs
	}
	var b strings.Builder
	for i := start; i < len(history); i++ {
		pair := history[i]
		if strings.TrimSpace(pair.Source) == "" || strings.TrimSpace(pair.Translated) == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("Segment %d Source:\n%s\nSegment %d Translation:\n%s\n\n", i+1, pair.Source, i+1, pair.Translated))
		if b.Len() > maxChars {
			break
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return "(none)"
	}
	if len(text) > maxChars {
		return text[len(text)-maxChars:]
	}
	return text
}

func (s *Server) loadAISettings() (aiSettings, error) {
	get := func(key string) (string, error) {
		value, ok, err := s.store.GetSetting(key)
		if err != nil {
			return "", err
		}
		if ok {
			return strings.TrimSpace(value), nil
		}
		return "", nil
	}

	apiKey, err := get(settingKeyAIApiKey)
	if err != nil {
		return aiSettings{}, err
	}
	baseURL, err := get(settingKeyAIBaseURL)
	if err != nil {
		return aiSettings{}, err
	}
	model, err := get(settingKeyAIModel)
	if err != nil {
		return aiSettings{}, err
	}
	targetLang, err := get(settingKeyAITargetLang)
	if err != nil {
		return aiSettings{}, err
	}

	if baseURL == "" {
		baseURL = defaultAIBaseURL
	}
	if model == "" {
		model = defaultAIModel
	}
	if targetLang == "" {
		targetLang = defaultAITargetLang
	}

	return aiSettings{APIKey: apiKey, BaseURL: baseURL, Model: model, TargetLang: targetLang}, nil
}
