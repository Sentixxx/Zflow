package handler

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"time"
)

type profileFolderRecord struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

type profileFeedRecord struct {
	URL              string `json:"url"`
	Title            string `json:"title"`
	FolderID         *int64 `json:"folder_id,omitempty"`
	CustomScript     string `json:"custom_script,omitempty"`
	CustomScriptLang string `json:"custom_script_lang,omitempty"`
}

type profileExportPayload struct {
	Version    string                `json:"version"`
	ExportedAt string                `json:"exported_at"`
	Folders    []profileFolderRecord `json:"folders"`
	Feeds      []profileFeedRecord   `json:"feeds"`
}

type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    opmlBody `xml:"body"`
}

type opmlHead struct {
	Title string `xml:"title"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Text     string        `xml:"text,attr,omitempty"`
	Title    string        `xml:"title,attr,omitempty"`
	Type     string        `xml:"type,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Outlines []opmlOutline `xml:"outline,omitempty"`
}

type storeFolderView struct {
	ID       int64
	Name     string
	ParentID *int64
}

func (s *Server) handleExportProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	folders := s.store.ListFolders()
	feeds := s.store.List()

	outFolders := make([]profileFolderRecord, 0, len(folders))
	for _, f := range folders {
		outFolders = append(outFolders, profileFolderRecord{ID: f.ID, Name: f.Name, ParentID: f.ParentID})
	}
	outFeeds := make([]profileFeedRecord, 0, len(feeds))
	for _, f := range feeds {
		outFeeds = append(outFeeds, profileFeedRecord{
			URL:              f.URL,
			Title:            f.Title,
			FolderID:         f.FolderID,
			CustomScript:     f.CustomScript,
			CustomScriptLang: normalizeScriptLang(f.CustomScriptLang),
		})
	}
	payload := profileExportPayload{
		Version:    "zflow-profile-v1",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Folders:    outFolders,
		Feeds:      outFeeds,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="zflow-profile.json"`)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleImportProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var payload profileExportPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	folderMap := make(map[int64]int64)
	pending := append([]profileFolderRecord(nil), payload.Folders...)
	for len(pending) > 0 {
		progress := false
		rest := make([]profileFolderRecord, 0, len(pending))
		for _, f := range pending {
			var parentID *int64
			if f.ParentID != nil {
				mapped, ok := folderMap[*f.ParentID]
				if !ok {
					rest = append(rest, f)
					continue
				}
				parentID = &mapped
			}
			created, err := s.store.CreateFolder(f.Name, parentID)
			if err != nil {
				rest = append(rest, f)
				continue
			}
			folderMap[f.ID] = created.ID
			progress = true
		}
		if !progress {
			break
		}
		pending = rest
	}

	importedFeeds := 0
	updatedFeeds := 0
	for _, fr := range payload.Feeds {
		if strings.TrimSpace(fr.URL) == "" {
			continue
		}
		var newFolderID *int64
		if fr.FolderID != nil {
			if mapped, ok := folderMap[*fr.FolderID]; ok {
				newFolderID = &mapped
			}
		}

		existing, ok, err := s.store.GetFeedByURL(fr.URL)
		if err != nil {
			continue
		}
		if !ok {
			created, err := s.store.CreateFeedPlaceholder(fr.URL, fr.Title, newFolderID)
			if err != nil {
				continue
			}
			if strings.TrimSpace(fr.CustomScript) != "" {
				_, _, _ = s.store.UpdateFeedScript(created.ID, fr.CustomScript, normalizeScriptLang(fr.CustomScriptLang))
			}
			importedFeeds++
			continue
		}

		if strings.TrimSpace(fr.Title) != "" && fr.Title != existing.Title {
			_, _, _ = s.store.UpdateFeedTitle(existing.ID, fr.Title)
		}
		_, _, _ = s.store.UpdateFeedFolder(existing.ID, newFolderID)
		_, _, _ = s.store.UpdateFeedScript(existing.ID, fr.CustomScript, normalizeScriptLang(fr.CustomScriptLang))
		updatedFeeds++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"imported_feeds":   importedFeeds,
		"updated_feeds":    updatedFeeds,
		"imported_folders": len(folderMap),
	})
}

func (s *Server) handleExportOPML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	folders := s.store.ListFolders()
	feeds := s.store.List()

	root := make([]opmlOutline, 0)
	foldersByParent := make(map[int64][]storeFolderView)
	rootFolders := make([]storeFolderView, 0)
	feedsByFolder := make(map[int64][]opmlOutline)
	rootFeeds := make([]opmlOutline, 0)

	for _, f := range feeds {
		node := opmlOutline{Text: feedTitleFallback(f.Title, f.URL), Title: feedTitleFallback(f.Title, f.URL), Type: "rss", XMLURL: f.URL, HTMLURL: f.URL}
		if f.FolderID == nil {
			rootFeeds = append(rootFeeds, node)
		} else {
			feedsByFolder[*f.FolderID] = append(feedsByFolder[*f.FolderID], node)
		}
	}

	for _, f := range folders {
		v := storeFolderView{ID: f.ID, Name: f.Name, ParentID: f.ParentID}
		if f.ParentID == nil {
			rootFolders = append(rootFolders, v)
		} else {
			foldersByParent[*f.ParentID] = append(foldersByParent[*f.ParentID], v)
		}
	}

	var buildFolder func(storeFolderView) opmlOutline
	buildFolder = func(f storeFolderView) opmlOutline {
		node := opmlOutline{Text: f.Name, Title: f.Name}
		node.Outlines = append(node.Outlines, feedsByFolder[f.ID]...)
		for _, child := range foldersByParent[f.ID] {
			node.Outlines = append(node.Outlines, buildFolder(child))
		}
		return node
	}

	for _, rf := range rootFolders {
		root = append(root, buildFolder(rf))
	}
	root = append(root, rootFeeds...)

	doc := opmlDoc{Version: "2.0", Head: opmlHead{Title: "Zflow Subscriptions"}, Body: opmlBody{Outlines: root}}
	encoded, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate opml"})
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="zflow-subscriptions.opml"`)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(encoded)
}

func feedTitleFallback(title, rawURL string) string {
	if strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	return strings.TrimSpace(rawURL)
}

func (s *Server) handleImportOPML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var doc opmlDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid opml"})
		return
	}

	createdFolders := 0
	importedFeeds := 0
	updatedFeeds := 0

	var walk func([]opmlOutline, *int64)
	walk = func(nodes []opmlOutline, parentID *int64) {
		for _, node := range nodes {
			if strings.TrimSpace(node.XMLURL) != "" || strings.EqualFold(strings.TrimSpace(node.Type), "rss") {
				url := strings.TrimSpace(node.XMLURL)
				if url == "" {
					continue
				}
				title := strings.TrimSpace(node.Title)
				if title == "" {
					title = strings.TrimSpace(node.Text)
				}
				existing, ok, err := s.store.GetFeedByURL(url)
				if err != nil {
					continue
				}
				if !ok {
					_, err := s.store.CreateFeedPlaceholder(url, title, parentID)
					if err == nil {
						importedFeeds++
					}
					continue
				}
				if title != "" && title != existing.Title {
					_, _, _ = s.store.UpdateFeedTitle(existing.ID, title)
				}
				_, _, _ = s.store.UpdateFeedFolder(existing.ID, parentID)
				updatedFeeds++
				continue
			}

			name := strings.TrimSpace(node.Title)
			if name == "" {
				name = strings.TrimSpace(node.Text)
			}
			if name == "" {
				name = "Untitled Folder"
			}
			f, err := s.store.CreateFolder(name, parentID)
			var currentParent *int64 = parentID
			if err == nil {
				createdFolders++
				currentParent = &f.ID
			}
			walk(node.Outlines, currentParent)
		}
	}
	walk(doc.Body.Outlines, nil)

	writeJSON(w, http.StatusOK, map[string]any{
		"imported_feeds":   importedFeeds,
		"updated_feeds":    updatedFeeds,
		"imported_folders": createdFolders,
	})
}
