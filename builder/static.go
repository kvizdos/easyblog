package builder

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/kvizdos/easyblog/feed"
)

const templatedPageSuffix = ".tmpl.html"

// buildStaticFiles copies static/ into the output directory. Files ending in
// .tmpl.html are executed as templates (with partials and a Page) first.
// HTML pages are added to the sitemap, except 404.html.
func (b *Builder) buildStaticFiles() error {
	if b.Config.StaticConfig.Path == "" {
		return nil
	}
	staticDir := filepath.Join(b.Config.InputDirectory, b.Config.StaticConfig.Path)
	if _, err := os.Stat(staticDir); err != nil {
		return fmt.Errorf("static files: %w", err)
	}

	var errs []error
	err := filepath.WalkDir(staticDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(staticDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		templated := strings.HasSuffix(rel, templatedPageSuffix)
		if templated {
			rel = strings.TrimSuffix(rel, templatedPageSuffix) + ".html"
		}

		if !strings.HasSuffix(rel, ".html") {
			return copyFile(path, ensureDir(b.outPath(filepath.FromSlash(rel))))
		}

		stem := strings.TrimSuffix(rel, ".html")
		if stem == "index" || strings.HasSuffix(stem, "/index") {
			stem = strings.TrimSuffix(stem, "index")
		}
		sitePath := "/" + stem
		outFile := b.outPath(filepath.FromSlash(rel))
		if b.Config.CleanURLs && sitePath != "/404" && !strings.HasSuffix(sitePath, "/") {
			outFile = b.outFileForPath(sitePath)
		}
		slug := sitePath
		if !strings.HasSuffix(slug, "/") {
			slug = pageSlug(b.Config, slug)
		}

		if templated {
			if err := b.renderStaticTemplate(path, outFile, slug); err != nil {
				errs = append(errs, err)
				return nil
			}
		} else if err := copyFile(path, ensureDir(outFile)); err != nil {
			return err
		}

		if sitePath != "/404" && sitePath != "/" {
			b.sitemap.AddPage(slug, b.lastModified(path))
		}
		return nil
	})
	errs = append(errs, err)
	return errors.Join(errs...)
}

func ensureDir(path string) string {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return path
}

// pageTitle turns "/about-me" into "About Me". The site root has no title.
func pageTitle(slug string) string {
	words := strings.FieldsFunc(path.Base("/"+strings.Trim(slug, "/")), func(r rune) bool { return r == '-' || r == '_' || r == '/' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func (b *Builder) renderStaticTemplate(src, dst, slug string) error {
	t, err := b.parseTemplate(src)
	if err != nil {
		return err
	}
	var doc bytes.Buffer
	page := Page{Title: pageTitle(slug), Slug: slug, URL: b.Config.BaseURL + slug, Site: b.site}
	if err := t.Execute(&doc, page); err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	return writeFile(dst, doc.Bytes())
}

// lastModified returns the last commit time of path when Sitemap.GitLastMod
// is set (falling back to mtime), otherwise its mtime.
func (b *Builder) lastModified(path string) time.Time {
	if b.Config.Sitemap.GitLastMod {
		out, err := exec.Command("git", "-C", filepath.Dir(path), "log", "-1", "--format=%cI", "--", filepath.Base(path)).Output()
		if t, perr := time.Parse(time.RFC3339, strings.TrimSpace(string(out))); err == nil && perr == nil {
			return t
		}
	}
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func (b *Builder) buildFeeds(posts PostList) error {
	if !b.Config.Feed.Enabled {
		return nil
	}

	main := b.newFeed(posts, b.Config.Title, b.Config.Description, b.Config.BaseURL+"/", b.site.FeedURL)
	main.AtomURL = b.site.AtomURL
	rss, err := main.RSS()
	if err != nil {
		return err
	}
	atom, err := main.Atom()
	if err != nil {
		return err
	}
	errs := []error{
		writeFile(b.outPath("rss.xml"), rss),
		writeFile(b.outPath("atom.xml"), atom),
	}

	for key, tagged := range b.groupTags(posts) {
		title := key.Name
		if b.Config.Title != "" {
			title = fmt.Sprintf("%s - %s", key.Name, b.Config.Title)
		}
		feedURL := fmt.Sprintf("%s/tags/%s.xml", b.Config.BaseURL, key.Slug)
		f := b.newFeed(tagged, title, "Posts tagged "+key.Name, b.Config.BaseURL+b.tagSlug(key.Name), feedURL)
		out, err := f.RSS()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, writeFile(b.outPath("tags", key.Slug+".xml"), out))
	}
	return errors.Join(errs...)
}

func (b *Builder) newFeed(posts PostList, title, description, link, feedURL string) feed.Feed {
	f := feed.Feed{
		Title:       title,
		Description: description,
		Link:        link,
		FeedURL:     feedURL,
		Author:      b.Config.Author,
	}
	for i, p := range posts {
		if i >= b.Config.Feed.Limit {
			break
		}
		item := feed.Item{
			Title:      p.Title,
			Link:       p.URL,
			Summary:    p.Summary,
			Author:     p.Author,
			Categories: p.Tags,
			Published:  p.PublishedAt,
			Updated:    p.UpdatedAt,
		}
		if b.Config.Feed.FullContent {
			item.Content = feed.AbsolutizeHTML(string(b.parsedPosts[p.Slug].Body), b.Config.BaseURL, p.URL)
		}
		f.Items = append(f.Items, item)
	}
	return f
}

// generateOG renders a post's OG image, skipping it when the title and OG
// config are unchanged since the last build in this process (e.g. -serve).
func (b *Builder) generateOG(post Post) (err error) {
	outPath := b.outPath("og_images", post.OGName+".png")
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%#v", post.Title, b.Config.OGImageConfig)))
	hash := hex.EncodeToString(sum[:])

	b.ogMu.Lock()
	if b.ogCache == nil {
		b.ogCache = map[string]string{}
	}
	if b.ogFiles == nil {
		b.ogFiles = map[string]bool{}
	}
	b.ogFiles[outPath] = true
	cached := b.ogCache[outPath] == hash
	b.ogMu.Unlock()

	if cached {
		if _, err := os.Stat(outPath); err == nil {
			return nil
		}
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("posts/%s: generating OG image: %v", post.SourceFile, r)
		}
	}()
	if b.OGGenerator != nil {
		b.OGGenerator(post.Title, outPath, b.Config.OGImageConfig)
	} else {
		GenerateOG(post.Title, outPath, b.Config.OGImageConfig)
	}

	b.ogMu.Lock()
	b.ogCache[outPath] = hash
	b.ogMu.Unlock()
	return nil
}

// pruneOGImages deletes OG images for posts that no longer exist.
func (b *Builder) pruneOGImages() error {
	b.ogMu.Lock()
	defer b.ogMu.Unlock()

	dir := b.outPath("og_images")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if !b.ogFiles[path] {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
			delete(b.ogCache, path)
		}
	}
	b.ogFiles = map[string]bool{}
	return nil
}
