package builder

import (
	"fmt"
	"html/template"
	"maps"
	"slices"
	"strings"
	"time"
)

// Site is available to every template via the `site` func, and as .Site on
// posts, tag pages, and templated static pages.
type Site struct {
	BaseURL     string
	Title       string
	Description string
	Author      string
	BuildTime   time.Time
	Posts       PostList // all published posts, newest first
	Tags        []string // sorted
	FeedURL     string   // absolute URL of rss.xml, empty when feeds are disabled
	AtomURL     string
}

// TagPage is passed to tag.html.
type TagPage struct {
	Tag     string
	Posts   PostList
	Slug    string // e.g. /tags/go
	URL     string
	FeedURL string // per-tag RSS feed, empty when feeds are disabled
	Site    *Site
}

// Page is passed to templated static pages (static/*.tmpl.html).
type Page struct {
	Title string // from the file name: about-me.tmpl.html -> "About Me"
	Slug  string
	URL   string
	Site  *Site
}

func (b *Builder) tagSlug(tag string) string {
	return pageSlug(b.Config, "/tags/"+slugify(tag))
}

func (b *Builder) getFuncsMap() template.FuncMap {
	out := template.FuncMap{
		"TagToURL": slugify,
		"slugify":  slugify,
		"contains": func(slice []string, item string) bool {
			return slices.Contains(slice, item)
		},
		"site": func() *Site {
			return b.site
		},
		"isoDate":    func(t time.Time) string { return t.Format("2006-01-02") },
		"rfc3339":    func(t time.Time) string { return t.Format(time.RFC3339) },
		"rfc1123":    func(t time.Time) string { return t.Format(time.RFC1123Z) },
		"formatDate": func(layout string, t time.Time) string { return t.Format(layout) },
		"absURL": func(p string) string {
			if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
				return p
			}
			return b.Config.BaseURL + "/" + strings.TrimPrefix(p, "/")
		},
		"seo": b.seo,
	}

	maps.Copy(out, b.CustomFuncs)
	return out
}

// seo emits <title>, description, canonical, OpenGraph, Twitter, and feed
// tags for a post, tag page, static page, or the index (PostList).
func (b *Builder) seo(v any) (template.HTML, error) {
	site := b.site
	var (
		title, desc, url, image, ogType = "", site.Description, site.BaseURL + "/", "", "website"
		published, modified             time.Time
		tags                            []string
		extraFeed                       string
	)

	switch p := v.(type) {
	case Post:
		title, desc, url, image, ogType = p.Title, p.Summary, p.URL, p.OGImageURL, "article"
		published, modified, tags = p.PublishedAt, p.UpdatedAt, p.Tags
	case *Post:
		return b.seo(*p)
	case TagPage:
		title, url, extraFeed = p.Tag, p.URL, p.FeedURL
		if site.Title != "" {
			desc = fmt.Sprintf("Posts tagged %s on %s", p.Tag, site.Title)
		}
	case *TagPage:
		return b.seo(*p)
	case Page:
		title, url = p.Title, p.URL
	case *Page:
		return b.seo(*p)
	case PostList, nil:
	default:
		return "", fmt.Errorf("seo: unsupported value %T", v)
	}

	fullTitle := site.Title
	if title != "" {
		fullTitle = title
		if site.Title != "" {
			fullTitle += " - " + site.Title
		}
	}

	var sb strings.Builder
	esc := template.HTMLEscapeString
	meta := func(attr, key, val string) {
		if val != "" {
			fmt.Fprintf(&sb, "<meta %s=\"%s\" content=\"%s\" />\n", attr, key, esc(val))
		}
	}

	fmt.Fprintf(&sb, "<title>%s</title>\n", esc(fullTitle))
	meta("name", "description", desc)
	fmt.Fprintf(&sb, "<link rel=\"canonical\" href=\"%s\" />\n", esc(url))
	meta("property", "og:title", firstNonEmpty(title, site.Title))
	meta("property", "og:type", ogType)
	meta("property", "og:url", url)
	meta("property", "og:description", desc)
	meta("property", "og:site_name", site.Title)
	if image != "" {
		meta("property", "og:image", image)
		meta("property", "og:image:type", "image/png")
		meta("property", "og:image:width", "1200")
		meta("property", "og:image:height", "630")
		meta("name", "twitter:card", "summary_large_image")
	} else {
		meta("name", "twitter:card", "summary")
	}
	if !published.IsZero() {
		meta("property", "article:published_time", published.Format(time.RFC3339))
		meta("property", "article:modified_time", modified.Format(time.RFC3339))
	}
	for _, t := range tags {
		meta("property", "article:tag", t)
	}
	if site.FeedURL != "" {
		fmt.Fprintf(&sb, "<link rel=\"alternate\" type=\"application/rss+xml\" title=\"%s\" href=\"%s\" />\n", esc(site.Title), esc(site.FeedURL))
		fmt.Fprintf(&sb, "<link rel=\"alternate\" type=\"application/atom+xml\" title=\"%s\" href=\"%s\" />\n", esc(site.Title), esc(site.AtomURL))
	}
	if extraFeed != "" {
		fmt.Fprintf(&sb, "<link rel=\"alternate\" type=\"application/rss+xml\" title=\"%s\" href=\"%s\" />\n", esc(fullTitle), esc(extraFeed))
	}

	return template.HTML(sb.String()), nil
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// relatedPosts ranks other posts by shared tag count, ties broken by recency.
func relatedPosts(post PostMetadata, all PostList, n int) PostList {
	if n <= 0 || len(post.Tags) == 0 {
		return PostList{}
	}
	type scored struct {
		meta  PostMetadata
		score int
	}
	var candidates []scored
	for _, other := range all {
		if other.Slug == post.Slug {
			continue
		}
		score := 0
		for _, t := range other.Tags {
			if slices.Contains(post.Tags, t) {
				score++
			}
		}
		if score > 0 {
			candidates = append(candidates, scored{other, score})
		}
	}
	// all is already newest-first, so a stable sort by score keeps recency as the tiebreak.
	slices.SortStableFunc(candidates, func(a, b scored) int { return b.score - a.score })

	out := PostList{}
	for i := 0; i < len(candidates) && i < n; i++ {
		out = append(out, candidates[i].meta)
	}
	return out
}
