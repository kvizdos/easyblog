package sitemap

import (
	"encoding/xml"
	"fmt"
	"sync"
)

type SitemapPage struct {
	XMLName  xml.Name `xml:"url"`
	Location string   `xml:"loc"`
}

type Sitemap struct {
	XMLName    xml.Name `xml:"urlset"`
	Xmlns      string   `xml:"xmlns,attr"`
	XmlnsXHTML string   `xml:"xmlns:xhtml,attr"`

	BaseURL string        `xml:"-"`
	Pages   []SitemapPage `xml:"urlset"`

	mu sync.Mutex
}

func (s *Sitemap) AddPageURL(pageURL string) {
	s.mu.Lock()
	s.Pages = append(s.Pages, SitemapPage{
		Location: fmt.Sprintf("%s%s", s.BaseURL, pageURL),
	})
	s.mu.Unlock()
}

func (s *Sitemap) Marshal() []byte {
	out, err := xml.MarshalIndent(s, " ", "  ")
	if err != nil {
		panic(err)
	}
	return out
}
