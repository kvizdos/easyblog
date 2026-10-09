package builder

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	figure "github.com/mangoumbrella/goldmark-figure"
	fences "github.com/stefanfritsch/goldmark-fences"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/anchor"
	"go.abhg.dev/goldmark/toc"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

const wordsPerMinute = 230

type customTexter struct{}

func (*customTexter) AnchorText(h *anchor.HeaderInfo) []byte {
	if h.Level == 1 {
		return nil
	}
	return []byte("#")
}

func newMarkdown(config Config) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // read note
		),
		goldmark.WithExtensions(extension.GFM, meta.Meta, figure.Figure, &anchor.Extender{
			Attributer: anchor.Attributes{
				"class": "headerPermalink",
			},
			Texter: &customTexter{},
		},
			&fences.Extender{},
			highlighting.NewHighlighting(
				highlighting.WithStyle(config.CodeStyle),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
				),
			)),
	)
}

// ParsePost reads posts/<fileName>, renders it, and validates its frontmatter.
// Warnings (e.g. missing alt text) are returned separately from errors.
func ParsePost(config Config, fileName string) (post Post, warnings []string, err error) {
	relPath := filepath.Join("posts", fileName)
	postMd, err := os.ReadFile(filepath.Join(config.InputDirectory, relPath))
	if err != nil {
		return Post{}, nil, err
	}

	md := newMarkdown(config)
	context := parser.NewContext()
	doc := md.Parser().Parse(text.NewReader(postMd), parser.WithContext(context))
	metaData, err := meta.TryGet(context)
	if err != nil {
		return Post{}, nil, fmt.Errorf("%s: invalid frontmatter: %w", relPath, err)
	}
	if metaData == nil {
		metaData = map[string]any{}
	}

	var errs []error
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("%s: "+format, append([]any{relPath}, args...)...))
	}

	strippedFileName := strings.TrimSuffix(fileName, ".md")

	m := PostMetadata{
		RawMetadata:  metaData,
		Title:        strings.ReplaceAll(strippedFileName, "-", " "),
		Syndications: map[string]string{},
		SourceFile:   fileName,
	}

	if v, ok := metaData["Title"]; ok {
		if s, ok := v.(string); ok {
			m.Title = s
		} else {
			fail("Title must be a string, got %T", v)
		}
	}

	if v, ok := metaData["Draft"]; ok {
		if b, ok := v.(bool); ok {
			m.Draft = b
		} else {
			fail("Draft must be true or false, got %v", v)
		}
	}

	if v, ok := metaData["Date"]; ok {
		if s, ok := v.(string); ok {
			m.Date = s
		}
		if t, err := parseDate(v); err == nil {
			m.PublishedAt = t
			if m.Date == "" {
				m.Date = t.Format("01/02/2006")
			}
		} else {
			fail("Date: %v", err)
		}
	} else if m.Draft {
		m.PublishedAt = time.Now().UTC().Truncate(24 * time.Hour)
		m.Date = m.PublishedAt.Format("01/02/2006")
	} else {
		fail("missing Date")
	}

	m.UpdatedAt = m.PublishedAt
	if v, ok := metaData["Updated"]; ok {
		if t, err := parseDate(v); err == nil {
			m.UpdatedAt = t
		} else {
			fail("Updated: %v", err)
		}
	}

	if v, ok := metaData["Summary"]; !ok {
		fail("missing Summary")
	} else if s, ok := v.(string); ok {
		m.Summary = s
	} else {
		fail("Summary must be a string, got %T (wrap it in quotes)", v)
	}

	if v, ok := metaData["Author"]; ok {
		if s, ok := v.(string); ok {
			m.Author = s
		} else {
			fail("Author must be a string, got %T", v)
		}
	} else if config.Author != "" {
		m.Author = config.Author
	} else {
		fail("missing Author (or set Author in config)")
	}

	if v, ok := metaData["Tags"]; ok {
		tags, err := stringList(v)
		if err != nil {
			fail("Tags: %v", err)
		}
		m.Tags = tags
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}

	if v, ok := metaData["Aliases"]; ok {
		aliases, err := stringList(v)
		if err != nil {
			fail("Aliases: %v", err)
		}
		for _, a := range aliases {
			m.Aliases = append(m.Aliases, "/"+strings.Trim(a, "/"))
		}
	}

	if v, ok := metaData["Syndications"]; ok {
		// Accept a map, or a list of single-entry maps ("- bluesky: https://...").
		var maps []map[any]any
		switch v := v.(type) {
		case map[any]any:
			maps = append(maps, v)
		case []any:
			for _, item := range v {
				if m, ok := item.(map[any]any); ok {
					maps = append(maps, m)
				} else {
					fail("Syndications list items must be name: URL, got %v", item)
				}
			}
		default:
			fail("Syndications must be a map of name: URL, got %T", v)
		}
		for _, syn := range maps {
			for k, v := range syn {
				ks, kok := k.(string)
				vs, vok := v.(string)
				if !kok || !vok {
					fail("Syndications must map names to URLs, got %v: %v", k, v)
					continue
				}
				m.Syndications[ks] = vs
			}
		}
	}

	slugOverride := ""
	if v, ok := metaData["Slug"]; ok {
		if s, ok := v.(string); ok && strings.Trim(s, "/ ") != "" {
			slugOverride = s
		} else {
			fail("Slug must be a non-empty string, got %v", v)
		}
	}
	slugPath := postPath(config, strippedFileName, slugOverride)
	m.OGName = path.Base(slugPath)
	m.Slug = pageSlug(config, slugPath)
	m.URL = config.BaseURL + m.Slug
	m.OGImageURL = fmt.Sprintf("%s/og_images/%s.png", config.BaseURL, m.OGName)

	// Walk the AST once for headings and images.
	err = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.Heading:
			if n.Level >= 2 && n.Level <= 3 {
				m.HeadingCount++
			}
		case *ast.Image:
			if strings.TrimSpace(nodeText(n, postMd)) == "" {
				msg := fmt.Sprintf("%s: image %s has no alt text", relPath, n.Destination)
				if config.StrictAlt {
					errs = append(errs, errors.New(msg))
				} else {
					warnings = append(warnings, msg)
				}
			}
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return Post{}, nil, err
	}

	m.WordCount = countWords(postMd)
	m.ReadingMinutes = max(1, int(math.Ceil(float64(m.WordCount)/wordsPerMinute)))

	if len(errs) > 0 {
		return Post{}, warnings, errors.Join(errs...)
	}

	tocHTML := template.HTML("")
	if m.HeadingCount >= config.ToCMinHeadings {
		tree, err := toc.Inspect(doc, postMd, toc.MinDepth(2), toc.MaxDepth(3))
		if err != nil {
			return Post{}, warnings, fmt.Errorf("%s: building ToC: %w", relPath, err)
		}
		if list := toc.RenderList(tree); list != nil {
			var tocBuff bytes.Buffer
			if err := md.Renderer().Render(&tocBuff, []byte{}, list); err != nil {
				return Post{}, warnings, fmt.Errorf("%s: rendering ToC: %w", relPath, err)
			}
			tocHTML = template.HTML(tocBuff.String())
		}
	}

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, postMd, doc); err != nil {
		return Post{}, warnings, fmt.Errorf("%s: rendering: %w", relPath, err)
	}

	return Post{
		PostMetadata: m,
		Body:         template.HTML(buf.String()),
		ToC:          tocHTML,
	}, warnings, nil
}

// postPath returns the extensionless site path for a post, e.g. /post/My-Post.
func postPath(config Config, strippedFileName, slugOverride string) string {
	switch {
	case strings.HasPrefix(slugOverride, "/"):
		return "/" + strings.Trim(slugOverride, "/")
	case slugOverride != "":
		return "/post/" + strings.Trim(slugOverride, "/")
	case config.SlugStyle == SlugStyleSlugify:
		return "/post/" + slugify(strippedFileName)
	default:
		return "/post/" + strippedFileName
	}
}

// pageSlug turns an extensionless path into the public URL path. With
// CleanURLs, pages live at <path>/index.html, which hosts serve at <path>/.
func pageSlug(config Config, p string) string {
	if config.CleanURLs && p != "/" {
		return p + "/"
	}
	return p
}

// stringList accepts "a, b,c" or a YAML list and returns trimmed, non-empty items.
func stringList(v any) ([]string, error) {
	var parts []string
	switch v := v.(type) {
	case string:
		parts = strings.Split(v, ",")
	case []any:
		for _, item := range v {
			switch item.(type) {
			case string, int, float64, bool:
				parts = append(parts, fmt.Sprint(item))
			default:
				return nil, fmt.Errorf("list items must be strings, got %T", item)
			}
		}
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("must be a comma-separated string or a list, got %T", v)
	}

	out := []string{}
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

// nodeText concatenates the text of n's descendants (used for image alt text).
func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			b.Write(c.Segment.Value(src))
		case *ast.String:
			b.Write(c.Value)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

// countWords counts words in the markdown source, skipping frontmatter and
// fenced code blocks.
func countWords(src []byte) int {
	lines := strings.Split(string(src), "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}

	count := 0
	fence := ""
	for _, line := range lines[start:] {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:3]
			continue
		}
		for _, word := range strings.Fields(line) {
			if strings.IndexFunc(word, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
				count++
			}
		}
	}
	return count
}
