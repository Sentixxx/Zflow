package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	readability "github.com/go-shiori/go-readability"
)

func stripHTMLTags(raw string) string {
	re := regexp.MustCompile(`(?s)<[^>]*>`)
	stripped := re.ReplaceAllString(raw, " ")
	normalized := html.UnescapeString(stripped)
	return strings.Join(strings.Fields(normalized), " ")
}

func (s *Server) fetchReadableContent(ctx context.Context, rawURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Zflow/0.1 (+https://github.com/Sentixxx/Zflow)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.Contains(contentType, "application/pdf") || bytes.HasPrefix(rawBody, []byte("%PDF-")) {
		return "", errors.New("unsupported readability content type: pdf")
	}
	doc, err := readability.FromReader(bytes.NewReader(rawBody), parsedURL)
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		content = strings.TrimSpace(doc.TextContent)
	}
	if content == "" {
		return "", errors.New("empty readable content")
	}
	return content, nil
}
