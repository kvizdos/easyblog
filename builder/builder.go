package builder

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kvizdos/easyblog/sitemap"
)

type OGGeneratorFunc func(postTitle string, outPath string, config OGImageConfig)

type Builder struct {
	MaxConcurrentPageBuilds int
	Config                  Config
	CustomFuncs             template.FuncMap
	OGGenerator             OGGeneratorFunc

	IncludeDrafts bool // build Draft: true posts (their pages only)
	IncludeFuture bool // build future-dated posts even with SkipFuturePosts

	buildMu sync.Mutex

	site *Site

	postTemplate  *template.Template
	indexTemplate *template.Template
	tagTemplate   *template.Template

	sitemap     *sitemap.Sitemap
	parsedPosts map[string]Post // by slug, includes drafts

	ogMu    sync.Mutex
	ogCache map[string]string // OG image path -> hash of its inputs
	ogFiles map[string]bool   // OG images written by the current build
}

var (
	debounceMu sync.Mutex
	debounce   *time.Timer
)

const debounceDelay = 250 * time.Millisecond

// helper to walk all subdirs and add them to the watcher
func watchRecursive(watcher *fsnotify.Watcher, root, outDir string) error {
	absOut, _ := filepath.Abs(outDir)
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden dirs.
			if strings.HasPrefix(d.Name(), ".") && len(d.Name()) > 1 {
				return filepath.SkipDir
			}
			// Skip the output dir and any of its subdirs
			if abs, _ := filepath.Abs(path); abs == absOut {
				return filepath.SkipDir
			}
			return watcher.Add(path)
		}
		return nil
	})
}

func (b *Builder) Serve(port string) {
	if err := b.Build(); err != nil {
		log.Println(err)
	}

	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}
		defer watcher.Close()

		err = watchRecursive(watcher, b.Config.InputDirectory, b.Config.OutputDirectory)
		if err != nil {
			log.Fatal(err)
		}

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
					log.Println("File changed:", event.Name)

					debounceMu.Lock()
					if debounce != nil {
						debounce.Stop()
					}
					debounce = time.AfterFunc(debounceDelay, func() {
						log.Println("Running build...")
						if err := b.Build(); err != nil {
							log.Println(err)
						}
					})
					debounceMu.Unlock()
				}

				// If a new directory was created, watch it
				if event.Op&fsnotify.Create != 0 {
					fi, err := os.Stat(event.Name)
					if err == nil && fi.IsDir() {
						_ = watchRecursive(watcher, event.Name, b.Config.OutputDirectory)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Watcher error:", err)
			}
		}
	}()

	outDir := b.Config.OutputDirectory
	fileServer := http.FileServer(http.Dir(outDir))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(outDir, filepath.FromSlash(r.URL.Path))

		// Mimic GitHub Pages: /x serves x.html, /x/ serves x/index.html.
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if filepath.Ext(path) == "" {
				if _, err := os.Stat(path + ".html"); err == nil {
					r.URL.Path += ".html"
					fileServer.ServeHTTP(w, r)
					return
				}
			}
			if page, err := os.ReadFile(filepath.Join(outDir, "404.html")); err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				w.Write(page)
				return
			}
		}

		fileServer.ServeHTTP(w, r)
	})
	log.Println("Serving on http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Build renders the whole site into Config.OutputDirectory.
func (b *Builder) Build() error {
	b.buildMu.Lock()
	defer b.buildMu.Unlock()

	start := time.Now()
	if err := b.Config.Validate(); err != nil {
		return err
	}
	if b.MaxConcurrentPageBuilds <= 0 {
		b.MaxConcurrentPageBuilds = 5
	}

	all, drafts, err := b.parsePosts(start)
	if err != nil {
		return err
	}

	if err := b.parseTemplates(); err != nil {
		return err
	}

	b.site = b.newSite(all, start)
	b.sitemap = &sitemap.Sitemap{
		BaseURL:    b.Config.BaseURL,
		Pages:      []sitemap.SitemapPage{},
		Xmlns:      "http://www.sitemaps.org/schemas/sitemap/0.9",
		XmlnsXHTML: "http://www.w3.org/1999/xhtml",
	}

	if err := b.setupOutDirectory(); err != nil {
		return err
	}

	var errs []error
	var errsMu sync.Mutex
	collect := func(err error) {
		if err != nil {
			errsMu.Lock()
			errs = append(errs, err)
			errsMu.Unlock()
		}
	}

	var wg sync.WaitGroup
	run := func(f func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			collect(f())
		}()
	}

	run(func() error { return b.buildPosts(all, drafts) })
	run(func() error { return b.buildIndexHTML(all) })
	run(func() error { return b.buildTagPages(all) })
	run(func() error { return b.buildFeeds(all) })
	run(func() error {
		withDrafts := slices.Clone(all)
		for _, d := range drafts {
			withDrafts = append(withDrafts, d.PostMetadata)
		}
		return b.buildAliases(withDrafts)
	})
	wg.Wait()

	// Static files go last so they can intentionally override generated files.
	collect(b.buildStaticFiles())
	collect(b.writeRobotsTxt())
	collect(b.writeSitemapToDisk())
	collect(b.pruneOGImages())

	if err := errors.Join(errs...); err != nil {
		return err
	}

	fmt.Printf("Built %d posts in %s\n", len(all)+len(drafts), time.Since(start))
	return nil
}

// parsePosts parses every post concurrently, then splits out drafts (only
// kept when IncludeDrafts) and drops future posts when configured to.
func (b *Builder) parsePosts(now time.Time) (published PostList, drafts []Post, err error) {
	postsDir := filepath.Join(b.Config.InputDirectory, "posts")
	files, err := os.ReadDir(postsDir)
	if err != nil {
		return nil, nil, fmt.Errorf("reading posts: %w", err)
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		posts    []Post
		errs     []error
		warnings []string
	)
	pool := make(chan struct{}, b.MaxConcurrentPageBuilds)
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".md") {
			continue
		}
		pool <- struct{}{}
		wg.Add(1)
		go func(fileName string) {
			defer func() {
				<-pool
				wg.Done()
			}()
			post, warns, err := ParsePost(b.Config, fileName)
			mu.Lock()
			defer mu.Unlock()
			warnings = append(warnings, warns...)
			if err != nil {
				errs = append(errs, err)
				return
			}
			posts = append(posts, post)
		}(file.Name())
	}
	wg.Wait()

	sort.Strings(warnings)
	for _, w := range warnings {
		fmt.Println("warning:", w)
	}

	// Detect two posts claiming the same URL (or an alias shadowing a post).
	owner := map[string]string{}
	claim := func(p, file string) {
		key := strings.TrimSuffix(p, "/")
		if other, ok := owner[key]; ok && other != file {
			errs = append(errs, fmt.Errorf("posts/%s: URL %s is already used by posts/%s", file, p, other))
		}
		owner[key] = file
	}
	slices.SortFunc(posts, func(a, b Post) int { return strings.Compare(a.SourceFile, b.SourceFile) })
	for _, p := range posts {
		claim(p.Slug, p.SourceFile)
	}
	for _, p := range posts {
		for _, a := range p.Aliases {
			claim(a, p.SourceFile)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}

	for _, p := range posts {
		switch {
		case p.Draft:
			if b.IncludeDrafts {
				fmt.Printf("draft: %s\n", p.Slug)
				drafts = append(drafts, p)
			}
		case b.Config.SkipFuturePosts && !b.IncludeFuture && p.PublishedAt.After(now):
			fmt.Printf("skipping future post: posts/%s (%s)\n", p.SourceFile, p.Date)
		default:
			published = append(published, p.PostMetadata)
		}
	}
	sort.Sort(published)

	// Keep the full Post (with Body) around for rendering.
	b.parsedPosts = map[string]Post{}
	for _, p := range posts {
		b.parsedPosts[p.Slug] = p
	}
	return published, drafts, nil
}

func (b *Builder) newSite(posts PostList, now time.Time) *Site {
	site := &Site{
		BaseURL:     b.Config.BaseURL,
		Title:       b.Config.Title,
		Description: b.Config.Description,
		Author:      b.Config.Author,
		BuildTime:   now,
		Posts:       posts,
		Tags:        []string{},
	}
	for tag := range b.groupTags(posts) {
		site.Tags = append(site.Tags, tag.Name)
	}
	slices.SortFunc(site.Tags, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	if b.Config.Feed.Enabled {
		site.FeedURL = b.Config.BaseURL + "/rss.xml"
		site.AtomURL = b.Config.BaseURL + "/atom.xml"
	}
	return site
}

type tagKey struct {
	Slug string
	Name string // display name, as first written
}

// groupTags groups posts by tag slug, so "Go" and "go" share a page.
func (b *Builder) groupTags(posts PostList) map[tagKey]PostList {
	bySlug := map[string]tagKey{}
	out := map[tagKey]PostList{}
	for _, post := range posts {
		for _, tag := range post.Tags {
			slug := slugify(tag)
			key, ok := bySlug[slug]
			if !ok {
				key = tagKey{Slug: slug, Name: tag}
				bySlug[slug] = key
			}
			out[key] = append(out[key], post)
		}
	}
	return out
}

func (b *Builder) outPath(parts ...string) string {
	return filepath.Join(append([]string{b.Config.OutputDirectory}, parts...)...)
}

// outFileForPath maps a site path (/post/X or /post/X/) to the file to write.
func (b *Builder) outFileForPath(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return b.outPath("index.html")
	}
	if b.Config.CleanURLs {
		return b.outPath(filepath.FromSlash(p), "index.html")
	}
	return b.outPath(filepath.FromSlash(p) + ".html")
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// setupOutDirectory empties the output directory (keeping og_images so
// unchanged OG images can be reused) and copies assets.
func (b *Builder) setupOutDirectory() error {
	outDir := b.Config.OutputDirectory
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "og_images" && entry.IsDir() {
			continue
		}
		if err := os.RemoveAll(filepath.Join(outDir, entry.Name())); err != nil {
			return err
		}
	}

	for _, dir := range []string{"og_images", "assets", "post", "tags"} {
		if err := os.MkdirAll(filepath.Join(outDir, dir), 0755); err != nil {
			return err
		}
	}

	assetsSrc := filepath.Join(b.Config.InputDirectory, "assets")
	if _, err := os.Stat(assetsSrc); os.IsNotExist(err) {
		return nil
	}
	if err := copyDir(assetsSrc, filepath.Join(outDir, "assets")); err != nil {
		return fmt.Errorf("copying assets: %w", err)
	}
	return nil
}

func (b *Builder) buildPosts(all PostList, drafts []Post) error {
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	pool := make(chan struct{}, b.MaxConcurrentPageBuilds)
	render := func(post Post, published bool) {
		defer func() {
			<-pool
			wg.Done()
		}()
		if err := b.buildPost(post, all, published); err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}
	}

	for _, meta := range all {
		pool <- struct{}{}
		wg.Add(1)
		go render(b.parsedPosts[meta.Slug], true)
	}
	for _, draft := range drafts {
		pool <- struct{}{}
		wg.Add(1)
		go render(draft, false)
	}
	wg.Wait()
	return errors.Join(errs...)
}

func (b *Builder) buildPost(post Post, all PostList, published bool) error {
	post.Site = b.site
	if published {
		i := slices.IndexFunc(all, func(m PostMetadata) bool { return m.Slug == post.Slug })
		if i > 0 {
			next := all[i-1]
			post.Next = &next
		}
		if i >= 0 && i < len(all)-1 {
			prev := all[i+1]
			post.Prev = &prev
		}
		post.Related = relatedPosts(post.PostMetadata, all, b.Config.RelatedPosts)
		b.sitemap.AddPage(post.Slug, post.UpdatedAt)
	}

	var doc bytes.Buffer
	if err := b.postTemplate.Execute(&doc, post); err != nil {
		return fmt.Errorf("posts/%s: %w", post.SourceFile, err)
	}
	post.HTML = doc.Bytes()

	if err := writeFile(b.outFileForPath(post.Slug), post.HTML); err != nil {
		return err
	}
	return b.generateOG(post)
}

func (b *Builder) buildIndexHTML(posts PostList) error {
	b.sitemap.AddPage("/", newest(posts))

	var doc bytes.Buffer
	if err := b.indexTemplate.Execute(&doc, posts); err != nil {
		return fmt.Errorf("index.html: %w", err)
	}
	return writeFile(b.outPath("index.html"), doc.Bytes())
}

func (b *Builder) buildTagPages(posts PostList) error {
	var errs []error
	for key, tagged := range b.groupTags(posts) {
		page := TagPage{
			Tag:   key.Name,
			Posts: tagged,
			Slug:  b.tagSlug(key.Name),
			Site:  b.site,
		}
		page.URL = b.Config.BaseURL + page.Slug
		if b.Config.Feed.Enabled {
			page.FeedURL = fmt.Sprintf("%s/tags/%s.xml", b.Config.BaseURL, key.Slug)
		}

		var doc bytes.Buffer
		if err := b.tagTemplate.Execute(&doc, page); err != nil {
			errs = append(errs, fmt.Errorf("tag.html (%s): %w", key.Name, err))
			continue
		}
		b.sitemap.AddPage(page.Slug, newest(tagged))
		errs = append(errs, writeFile(b.outFileForPath(page.Slug), doc.Bytes()))
	}
	return errors.Join(errs...)
}

// buildAliases writes a redirect page at each post's old URLs.
func (b *Builder) buildAliases(posts PostList) error {
	var errs []error
	for _, post := range posts {
		for _, alias := range post.Aliases {
			alias = strings.TrimSuffix(alias, ".html")
			target := template.HTMLEscapeString(post.URL)
			page := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>Redirecting to %s</title>
<link rel="canonical" href="%s" />
<meta name="robots" content="noindex" />
<meta http-equiv="refresh" content="0; url=%s" />
</head>
<body><a href="%s">This page has moved.</a></body>
</html>
`, template.HTMLEscapeString(post.Title), target, target, target)
			errs = append(errs, writeFile(b.outFileForPath(alias), []byte(page)))
		}
	}
	return errors.Join(errs...)
}

func newest(posts PostList) time.Time {
	var t time.Time
	for _, p := range posts {
		if p.UpdatedAt.After(t) {
			t = p.UpdatedAt
		}
	}
	return t
}

func (b *Builder) writeSitemapToDisk() error {
	exclude := map[string]bool{}
	for _, p := range b.Config.Sitemap.Exclude {
		exclude[strings.TrimSuffix(b.Config.BaseURL+"/"+strings.Trim(p, "/"), "/")] = true
	}
	pages := b.sitemap.Pages[:0]
	for _, p := range b.sitemap.Pages {
		if !exclude[strings.TrimSuffix(p.Location, "/")] {
			pages = append(pages, p)
		}
	}
	b.sitemap.Pages = pages
	return writeFile(b.outPath("sitemap.xml"), b.sitemap.Marshal())
}

func (b *Builder) writeRobotsTxt() error {
	path := b.outPath("robots.txt")
	line := fmt.Sprintf("Sitemap: %s/sitemap.xml\n", b.Config.BaseURL)

	existing, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return writeFile(path, []byte("User-agent: *\nAllow: /\n\n"+line))
	}
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(string(existing)), "sitemap:") {
		return nil
	}
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		existing = append(existing, '\n')
	}
	return writeFile(path, append(existing, []byte("\n"+line)...))
}

// parseTemplate parses a template file along with templates/partials/*.html.
func (b *Builder) parseTemplate(path string) (*template.Template, error) {
	t, err := template.New(filepath.Base(path)).Funcs(b.getFuncsMap()).ParseFiles(path)
	if err != nil {
		return nil, err
	}
	partials := filepath.Join(b.Config.InputDirectory, "templates", "partials", "*.html")
	if matches, _ := filepath.Glob(partials); len(matches) > 0 {
		if t, err = t.ParseGlob(partials); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func (b *Builder) parseTemplates() error {
	dir := filepath.Join(b.Config.InputDirectory, "templates")
	var errs []error
	for name, dst := range map[string]**template.Template{
		"post.html":  &b.postTemplate,
		"index.html": &b.indexTemplate,
		"tag.html":   &b.tagTemplate,
	} {
		t, err := b.parseTemplate(filepath.Join(dir, name))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		*dst = t
	}
	return errors.Join(errs...)
}
