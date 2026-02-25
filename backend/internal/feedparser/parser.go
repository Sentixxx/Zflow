package feedparser

import (
	"encoding/xml"
	"errors"
	"regexp"
	"strings"
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
		} `xml:"link"`
	} `xml:"entry"`
}

type ParsedItem struct {
	Title       string
	Link        string
	Summary     string
	CoverURL    string
	PublishedAt string
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
			Title:       strings.TrimSpace(it.Title),
			Link:        strings.TrimSpace(it.Link),
			Summary:     strings.TrimSpace(it.Description),
			CoverURL:    extractMediaCoverURL(it.RawXML),
			PublishedAt: strings.TrimSpace(it.PubDate),
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
		link := ""
		if len(entry.Link) > 0 {
			link = strings.TrimSpace(entry.Link[0].Href)
		}
		published := strings.TrimSpace(entry.Published)
		if published == "" {
			published = strings.TrimSpace(entry.Updated)
		}

		items = append(items, ParsedItem{
			Title:       strings.TrimSpace(entry.Title),
			Link:        link,
			Summary:     strings.TrimSpace(entry.Summary),
			CoverURL:    extractMediaCoverURL(entry.RawXML),
			PublishedAt: published,
		})
	}

	return ParsedFeed{
		Title:     strings.TrimSpace(parsed.Title),
		Items:     items,
		IconHints: extractFeedIconHints(raw),
	}, nil
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
