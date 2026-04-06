package feedparser

import (
	"encoding/xml"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/model"
)

var ErrUnsupportedFeed = errors.New("unsupported feed format")

type root struct {
	XMLName xml.Name
}

type rss struct {
	Channel struct {
		Title string `xml:"title"`
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			Description string `xml:"description"`
			PubDate     string `xml:"pubDate"`
			RawXML      string `xml:",innerxml"`
		} `xml:"item"`
	} `xml:"channel"`
}

type atom struct {
	Title string `xml:"title"`
	Entry []struct {
		Title     string `xml:"title"`
		Summary   string `xml:"summary"`
		Published string `xml:"published"`
		Updated   string `xml:"updated"`
		RawXML    string `xml:",innerxml"`
		Link      []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

type ParsedItem struct {
	Title       string
	Link        string
	Summary     string
	CoverURL    string
	PublishedAt string
	SourcePayload *model.ArticleSourcePayload
}

type ParsedFeed struct {
	Title     string
	Items     []ParsedItem
	IconHints []string
}

func ParseFeed(raw []byte) (ParsedFeed, error) {
	var r root
	if err := xml.Unmarshal(raw, &r); err != nil {
		return ParsedFeed{}, err
	}

	switch strings.ToLower(r.XMLName.Local) {
	case "rss":
		return parseRSS(raw)
	case "feed":
		return parseAtom(raw)
	default:
		return ParsedFeed{}, ErrUnsupportedFeed
	}
}

func parseRSS(raw []byte) (ParsedFeed, error) {
	var parsed rss
	if err := xml.Unmarshal(raw, &parsed); err != nil {
		return ParsedFeed{}, err
	}

	items := make([]ParsedItem, 0, len(parsed.Channel.Items))
	for _, it := range parsed.Channel.Items {
		items = append(items, ParsedItem{
			Title:         strings.TrimSpace(it.Title),
			Link:          strings.TrimSpace(it.Link),
			Summary:       strings.TrimSpace(it.Description),
			CoverURL:      extractMediaCoverURL(it.RawXML),
			PublishedAt:   strings.TrimSpace(it.PubDate),
			SourcePayload: buildSourcePayload("rss", strings.TrimSpace(it.Title), strings.TrimSpace(it.Link), strings.TrimSpace(it.Description), strings.TrimSpace(it.PubDate), it.RawXML),
		})
	}

	return ParsedFeed{
		Title:     strings.TrimSpace(parsed.Channel.Title),
		Items:     items,
		IconHints: extractFeedIconHints(raw),
	}, nil
}

func parseAtom(raw []byte) (ParsedFeed, error) {
	var parsed atom
	if err := xml.Unmarshal(raw, &parsed); err != nil {
		return ParsedFeed{}, err
	}

	items := make([]ParsedItem, 0, len(parsed.Entry))
	for _, entry := range parsed.Entry {
		link := atomBestLink(entry.Link)
		published := strings.TrimSpace(entry.Published)
		if published == "" {
			published = strings.TrimSpace(entry.Updated)
		}

		items = append(items, ParsedItem{
			Title:         strings.TrimSpace(entry.Title),
			Link:          link,
			Summary:       strings.TrimSpace(entry.Summary),
			CoverURL:      extractMediaCoverURL(entry.RawXML),
			PublishedAt:   published,
			SourcePayload: buildSourcePayload("atom", strings.TrimSpace(entry.Title), link, strings.TrimSpace(entry.Summary), published, entry.RawXML),
		})
	}

	return ParsedFeed{
		Title:     strings.TrimSpace(parsed.Title),
		Items:     items,
		IconHints: extractFeedIconHints(raw),
	}, nil
}

// atomBestLink picks the best link from Atom <link> elements.
// Prefers rel="alternate" (the article page), then links with no rel
// (default is alternate per Atom spec), then falls back to the first link.
func atomBestLink(links []struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}) string {
	var fallback string
	for _, l := range links {
		href := strings.TrimSpace(l.Href)
		if href == "" {
			continue
		}
		rel := strings.ToLower(strings.TrimSpace(l.Rel))
		if rel == "alternate" {
			return href
		}
		if rel == "" && fallback == "" {
			fallback = href
		}
	}
	if fallback != "" {
		return fallback
	}
	// Last resort: first non-empty href
	for _, l := range links {
		if href := strings.TrimSpace(l.Href); href != "" {
			return href
		}
	}
	return ""
}

var (
	reMediaTag = regexp.MustCompile(`(?is)<(?:media:)?(?:content|thumbnail)\b[^>]*>`)
	reImgTag   = regexp.MustCompile(`(?is)<enclosure\b[^>]*>`)
	reURLAttr  = regexp.MustCompile(`(?is)\burl\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+))`)
	reTypeAttr = regexp.MustCompile(`(?is)\btype\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+))`)
	reIconTag  = regexp.MustCompile(`(?is)<icon>([^<]+)</icon>`)
	reLogoTag  = regexp.MustCompile(`(?is)<logo>([^<]+)</logo>`)
	reImageURL = regexp.MustCompile(`(?is)<image\b[^>]*>.*?<url>([^<]+)</url>.*?</image>`)
	reLinkIcon = regexp.MustCompile(`(?is)<(?:itunes:)?image\b[^>]*>`)
	reHrefAttr = regexp.MustCompile(`(?is)\bhref\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+))`)
	reStripTag = regexp.MustCompile(`(?s)<[^>]+>`)
)

func extractMediaCoverURL(rawItem string) string {
	trimmed := strings.TrimSpace(rawItem)
	if trimmed == "" {
		return ""
	}

	for _, tag := range reMediaTag.FindAllString(trimmed, -1) {
		urlValue := firstAttrMatch(reURLAttr, tag)
		if strings.TrimSpace(urlValue) != "" {
			return strings.TrimSpace(urlValue)
		}
	}

	for _, tag := range reImgTag.FindAllString(trimmed, -1) {
		contentType := strings.ToLower(firstAttrMatch(reTypeAttr, tag))
		if contentType != "" && !strings.HasPrefix(contentType, "image/") {
			continue
		}
		urlValue := firstAttrMatch(reURLAttr, tag)
		if strings.TrimSpace(urlValue) != "" {
			return strings.TrimSpace(urlValue)
		}
	}

	return ""
}

func firstAttrMatch(re *regexp.Regexp, input string) string {
	match := re.FindStringSubmatch(input)
	if len(match) == 0 {
		return ""
	}
	for i := 2; i < len(match); i++ {
		if strings.TrimSpace(match[i]) != "" {
			return strings.TrimSpace(match[i])
		}
	}
	return strings.TrimSpace(match[1])
}

func extractFeedIconHints(raw []byte) []string {
	text := string(raw)
	hints := make([]string, 0, 6)
	for _, re := range []*regexp.Regexp{reIconTag, reLogoTag, reImageURL} {
		matches := re.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			value := strings.TrimSpace(match[1])
			if value != "" {
				hints = append(hints, value)
			}
		}
	}
	for _, tag := range reLinkIcon.FindAllString(text, -1) {
		href := strings.TrimSpace(firstAttrMatch(reHrefAttr, tag))
		if href != "" {
			hints = append(hints, href)
		}
	}
	return uniqueStrings(hints)
}

type rawField struct {
	XMLName xml.Name
	Inner   string `xml:",innerxml"`
}

func buildSourcePayload(feedType string, title string, link string, summary string, publishedAt string, rawInnerXML string) *model.ArticleSourcePayload {
	payload := &model.ArticleSourcePayload{
		FeedType:    strings.TrimSpace(feedType),
		Title:       strings.TrimSpace(title),
		Link:        strings.TrimSpace(link),
		Summary:     strings.TrimSpace(summary),
		PublishedAt: strings.TrimSpace(publishedAt),
		Fields:      extractSourceFields(rawInnerXML),
	}
	if payload.Title == "" && payload.Link == "" && payload.Summary == "" && payload.PublishedAt == "" && len(payload.Fields) == 0 {
		return nil
	}
	return payload
}

func extractSourceFields(rawInnerXML string) []model.ArticleSourceField {
	trimmed := strings.TrimSpace(rawInnerXML)
	if trimmed == "" {
		return nil
	}

	decoder := xml.NewDecoder(strings.NewReader("<root>" + trimmed + "</root>"))
	fields := make([]model.ArticleSourceField, 0, 8)
	seen := make(map[string]int)
	depth := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch typed := token.(type) {
		case xml.StartElement:
			if depth == 0 && typed.Name.Local == "root" {
				depth++
				continue
			}
			if depth != 1 {
				depth++
				continue
			}

			var element rawField
			if err := decoder.DecodeElement(&element, &typed); err != nil {
				depth++
				continue
			}

			key := strings.ToLower(strings.TrimSpace(typed.Name.Local))
			depth = 1
			if shouldSkipSourceField(key) {
				continue
			}

			textValue := strings.TrimSpace(stripMarkup(element.Inner))
			htmlValue := unwrapCDATA(strings.TrimSpace(element.Inner))
			field := model.ArticleSourceField{Key: key}
			if looksLikeHTML(htmlValue) {
				field.ValueHTML = htmlValue
			} else {
				field.Value = textValue
			}
			if field.Value == "" && field.ValueHTML == "" {
				continue
			}

			seen[key]++
			if seen[key] > 1 {
				field.Key = key + "_" + strconv.Itoa(seen[key])
			}
			fields = append(fields, field)
		case xml.EndElement:
			if depth > 0 {
				depth--
			}
		}
	}

	return fields
}

func shouldSkipSourceField(key string) bool {
	switch key {
	case "title", "link", "description", "summary", "pubdate", "published", "updated":
		return true
	default:
		return false
	}
}

func looksLikeHTML(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return strings.Contains(trimmed, "<") && strings.Contains(trimmed, ">")
}

func stripMarkup(raw string) string {
	return strings.Join(strings.Fields(reStripTag.ReplaceAllString(raw, " ")), " ")
}

func unwrapCDATA(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "<![CDATA[") && strings.HasSuffix(trimmed, "]]>") {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<![CDATA["), "]]>"))
	}
	return trimmed
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
