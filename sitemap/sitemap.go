package sitemap

import (
	"encoding/xml"
	"fmt"
	"sort"
	"sync"
	"time"
)

type SitemapPage struct {
	XMLName  xml.Name `xml:"url"`
	Location string   `xml:"loc"`
	LastMod  string   `xml:"lastmod,omitempty"`
}

type Sitemap struct {
	XMLName    xml.Name `xml:"urlset"`
	Xmlns      string   `xml:"xmlns,attr"`
	XmlnsXHTML string   `xml:"xmlns:xhtml,attr"`

	BaseURL string        `xml:"-"`
	Pages   []SitemapPage `xml:"url"`

	mu sync.Mutex
}

func (s *Sitemap) AddPageURL(pageURL string) {
	s.AddPage(pageURL, time.Time{})
}

// AddPage adds a site-relative path with an optional last-modified time.
func (s *Sitemap) AddPage(pageURL string, lastMod time.Time) {
	page := SitemapPage{
		Location: fmt.Sprintf("%s%s", s.BaseURL, pageURL),
	}
	if !lastMod.IsZero() {
		page.LastMod = lastMod.Format("2006-01-02")
	}
	s.mu.Lock()
	s.Pages = append(s.Pages, page)
	s.mu.Unlock()
}

// Marshal renders the sitemap with pages sorted by location.
func (s *Sitemap) Marshal() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	sort.Slice(s.Pages, func(i, j int) bool { return s.Pages[i].Location < s.Pages[j].Location })
	out, err := xml.MarshalIndent(s, " ", "  ")
	if err != nil {
		panic(err)
	}
	return append([]byte(xml.Header), out...)
}
