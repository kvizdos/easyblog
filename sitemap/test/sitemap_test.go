package sitemap_test

import (
	"encoding/xml"
	"fmt"
	"testing"

	"github.com/kvizdos/easyblog/sitemap"
)

func TestMain(t *testing.T) {
	sm := &sitemap.Sitemap{
		Pages:      []sitemap.SitemapPage{},
		Xmlns:      "http://www.sitemaps.org/schemas/sitemap/0.9",
		XmlnsXHTML: "http://www.w3.org/1999/xhtml",
	}

	sm.AddPageURL("https://example.com/1")
	sm.AddPageURL("https://example.com/2")

	out, _ := xml.MarshalIndent(sm, " ", "  ")
	fmt.Println(string(out))
}
