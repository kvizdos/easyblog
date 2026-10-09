package builder

import (
	"html/template"
	"sort"
	"time"
)

// PostMetadata is everything about a post except its rendered body. It's what
// index.html and tag.html range over.
type PostMetadata struct {
	RawMetadata  map[string]any
	Slug         string // site-relative path, e.g. /post/My-Post
	URL          string // absolute URL, BaseURL + Slug
	OGName       string // base name of the OG image (and output file)
	OGImageURL   string
	Syndications map[string]string
	Title        string
	Date         string    // raw Date frontmatter, kept for back-compat
	PublishedAt  time.Time // parsed Date
	UpdatedAt    time.Time // parsed Updated, falls back to PublishedAt
	Summary      string
	Author       string
	Tags         []string
	Aliases      []string
	Draft        bool

	WordCount      int
	ReadingMinutes int
	HeadingCount   int

	SourceFile string // file name inside posts/
}

// Post is a fully rendered post, passed to post.html.
type Post struct {
	PostMetadata

	Body template.HTML
	HTML []byte
	ToC  template.HTML

	Prev    *PostMetadata // older post
	Next    *PostMetadata // newer post
	Related PostList

	Site *Site
}

type PostList []PostMetadata

// sort.Interface implementation: newest first.
func (p PostList) Len() int      { return len(p) }
func (p PostList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p PostList) Less(i, j int) bool {
	if !p[i].PublishedAt.Equal(p[j].PublishedAt) {
		return p[i].PublishedAt.After(p[j].PublishedAt)
	}
	return p[i].Slug < p[j].Slug
}

// Sorted returns a newest-first copy.
func (p PostList) Sorted() PostList {
	out := append(PostList(nil), p...)
	sort.Sort(out)
	return out
}

// Iterator returns a channel that iterates over sorted posts.
func (p PostList) Iterator() <-chan PostMetadata {
	ch := make(chan PostMetadata)
	go func() {
		// Assuming posts are already sorted.
		for _, post := range p {
			ch <- post
		}
		close(ch)
	}()
	return ch
}
