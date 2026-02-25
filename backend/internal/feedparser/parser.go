package feedparser

import (
	"encoding/xml"
	"errors"
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
		Link      []struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
	} `xml:"entry"`
}

type ParsedItem struct {
	Title       string
	Link        string
	Summary     string
	PublishedAt string
}

type ParsedFeed struct {
	Title string
	Items []ParsedItem
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
			PublishedAt: strings.TrimSpace(it.PubDate),
		})
	}

	return ParsedFeed{
		Title: strings.TrimSpace(parsed.Channel.Title),
		Items: items,
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
			PublishedAt: published,
		})
	}

	return ParsedFeed{
		Title: strings.TrimSpace(parsed.Title),
		Items: items,
	}, nil
}
