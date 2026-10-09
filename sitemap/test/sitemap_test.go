package sitemap_test

import (
	"os"
	"testing"
	"time"

	"github.com/kvizdos/easyblog/sitemap"
)

func TestMain(t *testing.T) {
	sm := &sitemap.Sitemap{
		BaseURL:    "https://example.com",
		Pages:      []sitemap.SitemapPage{},
		Xmlns:      "http://www.sitemaps.org/schemas/sitemap/0.9",
		XmlnsXHTML: "http://www.w3.org/1999/xhtml",
	}

	sm.AddPage("/2", time.Date(2025, 4, 21, 12, 0, 0, 0, time.UTC))
	sm.AddPageURL("/1")

	want, err := os.ReadFile("sitemap.xml")
	if err != nil {
		t.Fatal(err)
	}
	if got := sm.Marshal(); string(got) != string(want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
