// Package feed renders RSS 2.0 and Atom feeds.
package feed

import (
	"encoding/xml"
	"regexp"
	"strings"
	"time"
)

type Feed struct {
	Title       string
	Description string
	Link        string // site (or tag page) URL
	FeedURL     string // URL of this RSS feed
	AtomURL     string // URL of the Atom feed, if any
	Author      string
	Items       []Item // newest first
}

type Item struct {
	Title      string
	Link       string
	Summary    string
	Content    string // HTML; omitted when empty
	Author     string
	Categories []string
	Published  time.Time
	Updated    time.Time
}

// Updated is the newest item's update time.
func (f Feed) Updated() time.Time {
	var t time.Time
	for _, it := range f.Items {
		if it.Updated.After(t) {
			t = it.Updated
		}
	}
	return t
}

var relativeAttr = regexp.MustCompile(`(src|href|poster)="/([^/"][^"]*)?"`)
var fragmentHref = regexp.MustCompile(`href="#`)

// AbsolutizeHTML rewrites root-relative src/href attributes to absolute URLs
// and in-page anchors to pageURL#anchor, so content works in feed readers.
func AbsolutizeHTML(html, baseURL, pageURL string) string {
	html = relativeAttr.ReplaceAllString(html, `$1="`+baseURL+`/$2"`)
	return fragmentHref.ReplaceAllString(html, `href="`+pageURL+`#`)
}

type rss struct {
	XMLName   xml.Name `xml:"rss"`
	Version   string   `xml:"version,attr"`
	ContentNS string   `xml:"xmlns:content,attr"`
	AtomNS    string   `xml:"xmlns:atom,attr"`
	DCNS      string   `xml:"xmlns:dc,attr"`
	Channel   rssChannel
}

type rssChannel struct {
	XMLName       xml.Name  `xml:"channel"`
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	AtomLink      atomLink  `xml:"atom:link"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        rssGUID  `xml:"guid"`
	PubDate     string   `xml:"pubDate"`
	Author      string   `xml:"dc:creator,omitempty"`
	Description string   `xml:"description"`
	Content     *cdata   `xml:"content:encoded,omitempty"`
	Categories  []string `xml:"category"`
}

type rssGUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type cdata struct {
	Value string `xml:",cdata"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr,omitempty"`
}

// RSS renders the feed as RSS 2.0.
func (f Feed) RSS() ([]byte, error) {
	ch := rssChannel{
		Title:       f.Title,
		Link:        f.Link,
		Description: f.Description,
		AtomLink:    atomLink{Href: f.FeedURL, Rel: "self", Type: "application/rss+xml"},
	}
	if u := f.Updated(); !u.IsZero() {
		ch.LastBuildDate = u.Format(time.RFC1123Z)
	}
	for _, it := range f.Items {
		item := rssItem{
			Title:       it.Title,
			Link:        it.Link,
			GUID:        rssGUID{IsPermaLink: true, Value: it.Link},
			PubDate:     it.Published.Format(time.RFC1123Z),
			Author:      it.Author,
			Description: it.Summary,
			Categories:  it.Categories,
		}
		if it.Content != "" {
			item.Content = &cdata{strings.ReplaceAll(it.Content, "]]>", "]]]]><![CDATA[>")}
		}
		ch.Items = append(ch.Items, item)
	}

	doc := rss{
		Version:   "2.0",
		ContentNS: "http://purl.org/rss/1.0/modules/content/",
		AtomNS:    "http://www.w3.org/2005/Atom",
		DCNS:      "http://purl.org/dc/elements/1.1/",
		Channel:   ch,
	}
	return marshal(doc)
}

type atomFeed struct {
	XMLName  xml.Name    `xml:"feed"`
	Xmlns    string      `xml:"xmlns,attr"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle,omitempty"`
	ID       string      `xml:"id"`
	Links    []atomLink  `xml:"link"`
	Updated  string      `xml:"updated"`
	Author   *atomAuthor `xml:"author,omitempty"`
	Entries  []atomEntry `xml:"entry"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomEntry struct {
	Title      string         `xml:"title"`
	ID         string         `xml:"id"`
	Link       atomLink       `xml:"link"`
	Published  string         `xml:"published"`
	Updated    string         `xml:"updated"`
	Author     *atomAuthor    `xml:"author,omitempty"`
	Summary    string         `xml:"summary"`
	Content    *atomContent   `xml:"content,omitempty"`
	Categories []atomCategory `xml:"category"`
}

type atomContent struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

// Atom renders the feed as Atom 1.0.
func (f Feed) Atom() ([]byte, error) {
	doc := atomFeed{
		Xmlns:    "http://www.w3.org/2005/Atom",
		Title:    f.Title,
		Subtitle: f.Description,
		ID:       f.Link,
		Links: []atomLink{
			{Href: f.AtomURL, Rel: "self", Type: "application/atom+xml"},
			{Href: f.Link, Rel: "alternate", Type: "text/html"},
		},
		Updated: f.Updated().Format(time.RFC3339),
	}
	if f.Author != "" {
		doc.Author = &atomAuthor{f.Author}
	}
	for _, it := range f.Items {
		e := atomEntry{
			Title:     it.Title,
			ID:        it.Link,
			Link:      atomLink{Href: it.Link, Rel: "alternate", Type: "text/html"},
			Published: it.Published.Format(time.RFC3339),
			Updated:   it.Updated.Format(time.RFC3339),
			Summary:   it.Summary,
		}
		if it.Author != "" {
			e.Author = &atomAuthor{it.Author}
		}
		if it.Content != "" {
			e.Content = &atomContent{Type: "html", Value: it.Content}
		}
		for _, c := range it.Categories {
			e.Categories = append(e.Categories, atomCategory{c})
		}
		doc.Entries = append(doc.Entries, e)
	}
	return marshal(doc)
}

func marshal(v any) ([]byte, error) {
	out, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}
