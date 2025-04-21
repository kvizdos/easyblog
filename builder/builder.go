package builder

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kvizdos/easyblog/sitemap"
)

type Post struct {
	Title      string
	OGName     string
	Body       template.HTML
	HTML       []byte
	Date       string
	Summary    string
	OGImageURL string
	Author     string
	Tags       []string
	ToC        template.HTML
}

type PostMetadata struct {
	Slug    string
	Title   string
	Date    string
	Summary string
	Author  string
	Tags    []string
}

type PostList []PostMetadata

// sort.Interface implementation
func (p PostList) Len() int      { return len(p) }
func (p PostList) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p PostList) Less(i, j int) bool {
	// Use "01/02/2006" layout for parsing
	t1, err1 := time.Parse("01/02/2006", p[i].Date)
	t2, err2 := time.Parse("01/02/2006", p[j].Date)
	if err1 != nil || err2 != nil {
		return p[i].Date < p[j].Date
	}
	return t1.After(t2)
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

type Builder struct {
	MaxConcurrentPageBuilds int
	Config                  Config
	setupWaitGroup          sync.WaitGroup // setup things like parsing index.html, page.html

	staticFilesCreated sync.WaitGroup

	postTemplate  *template.Template
	indexTemplate *template.Template
	tagTemplate   *template.Template

	sitemap *sitemap.Sitemap
}

// helper to walk all subdirs and add them to the watcher
func watchRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "out" {
				return nil
			}
			return watcher.Add(path)
		}
		return nil
	})
}

func (b *Builder) Serve(port string) {
	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}
		defer watcher.Close()

		err = watchRecursive(watcher, b.Config.InputDirectory)
		if err != nil {
			log.Fatal(err)
		}

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("File changed:", event.Name)
				b.Build()

				// If a new directory was created, watch it
				if event.Op&fsnotify.Create != 0 {
					fi, err := os.Stat(event.Name)
					if err == nil && fi.IsDir() {
						_ = watchRecursive(watcher, event.Name)
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := "out" + r.URL.Path

		// Try to serve path as-is
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// If no extension and .html file exists, rewrite path
			if filepath.Ext(path) == "" {
				if _, err := os.Stat(path + ".html"); err == nil {
					r.URL.Path += ".html"
				}
			}
		}

		http.FileServer(http.Dir("out")).ServeHTTP(w, r)
	})
	log.Println("Serving on http://localhost:" + port)
	http.ListenAndServe(":"+port, nil)
}

func (b *Builder) Build() {
	now := time.Now()
	b.sitemap = &sitemap.Sitemap{
		BaseURL:    b.Config.BaseURL,
		Pages:      []sitemap.SitemapPage{},
		Xmlns:      "http://www.sitemaps.org/schemas/sitemap/0.9",
		XmlnsXHTML: "http://www.w3.org/1999/xhtml",
	}
	b.setupWaitGroup.Add(2)
	b.staticFilesCreated.Add(2)
	go b.setupHTML(b.Config.InputDirectory)
	go b.setupOutDirectory()

	postsChan, metadataChan := b.scanForMarkdownFiles(b.Config.InputDirectory)
	go b.buildIndexHTML(metadataChan)
	doneCh := b.buildPostHTML(postsChan)

	b.writePostOut(doneCh)

	b.staticFilesCreated.Wait()

	b.writeSitemapToDisk()
	b.buildStaticFiles()
	took := time.Now().Sub(now)

	fmt.Println("All done!", took)
}
func (b *Builder) setupOutDirectory() {
	defer b.setupWaitGroup.Done()
	outDir := "out"
	scaffoldDirs := []string{"og_images", "assets", "post", "tags"}

	// Ensure the out directory exists (or error if it conflicts with a non-directory)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		panic(err)
	}

	// Remove all files in the out directory (skip directories)
	entries, err := os.ReadDir(outDir)
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			fullPath := filepath.Join(outDir, entry.Name())
			if err := os.Remove(fullPath); err != nil {
				panic(err)
			}
		}
	}

	// Create scaffold directories inside "out"
	for _, dir := range scaffoldDirs {
		path := filepath.Join(outDir, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			panic(err)
		}
	}

	// Once the scaffold is ready, copy assets from Config.AssetsSource to out/assets.
	assetsSrc := filepath.Join(b.Config.InputDirectory, "assets")
	assetsDst := filepath.Join(outDir, "assets")
	if err := copyDir(assetsSrc, assetsDst); err != nil {
		panic(fmt.Sprintf("error copying assets: %v", err))
	}
}

func (b *Builder) scanForMarkdownFiles(inputDirectory string) (<-chan Post, <-chan PostMetadata) {
	postsDir := fmt.Sprintf("%s/posts/", inputDirectory)
	// Find all Markdown files
	files, err := os.ReadDir(postsDir)
	if err != nil {
		log.Fatalf("Error reading directory: %v", err)
	}

	postsChan := make(chan Post, 10)
	metadataChan := make(chan PostMetadata, 10)
	concurrentPageBuildsPool := make(chan struct{}, b.MaxConcurrentPageBuilds)
	go func() {
		defer close(postsChan)
		defer close(metadataChan)
		var wg sync.WaitGroup
		for _, file := range files {
			concurrentPageBuildsPool <- struct{}{}
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".md") {
				wg.Add(1)
				go func(fileName string) {
					defer func() {
						<-concurrentPageBuildsPool
						wg.Done()
					}()
					ParsePost(postsChan, metadataChan, b.Config, fileName)
				}(file.Name())
			}
		}
		wg.Wait()
	}()
	return postsChan, metadataChan
}

func (b *Builder) buildStaticFiles() {
	if b.Config.StaticConfig.Path == "" {
		return
	}
	staticDir := fmt.Sprintf("%s/%s", b.Config.InputDirectory, b.Config.StaticConfig.Path)

	copyDir(staticDir, "./out")
}

func (b *Builder) buildIndexHTML(metadata <-chan PostMetadata) {
	out := PostList{}

	for meta := range metadata {
		out = append(out, meta)
		b.sitemap.AddPageURL(meta.Slug)
	}

	b.setupWaitGroup.Wait()

	sort.Sort(out)

	go b.StartTagPageBuilder(out)

	var doc bytes.Buffer
	err := b.indexTemplate.Execute(&doc, out)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("./out/index.html", doc.Bytes(), 0644)
	if err != nil {
		panic(err)
	}

	b.staticFilesCreated.Done()
}

func (b *Builder) StartTagPageBuilder(posts PostList) {
	tagMap := map[string]PostList{}

	for post := range posts.Iterator() {
		for _, tag := range post.Tags {
			if v, ok := tagMap[tag]; ok {
				tagMap[tag] = append(v, post)
			} else {
				tagMap[tag] = PostList{post}
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(tagMap))
	for tagName, taggedPosts := range tagMap {
		go func() {
			defer wg.Done()
			b.buildTagHTML(tagName, taggedPosts)
		}()
	}
	wg.Wait()
	b.staticFilesCreated.Done()
}

func (b *Builder) buildTagHTML(tagName string, taggedPosts PostList) {
	var doc bytes.Buffer
	data := struct {
		Tag   string
		Posts PostList
	}{
		Tag:   tagName,
		Posts: taggedPosts,
	}
	err := b.tagTemplate.Execute(&doc, data)
	if err != nil {
		panic(err)
	}

	urlTag := strings.ToLower(tagName)
	urlTag = strings.ReplaceAll(urlTag, " ", "-")
	b.sitemap.AddPageURL(fmt.Sprintf("/tags/%s", urlTag))

	err = os.WriteFile(fmt.Sprintf("./out/tags/%s.html", urlTag), doc.Bytes(), 0644)
	if err != nil {
		panic(err)
	}
}

func (b *Builder) writeSitemapToDisk() {
	err := os.WriteFile("./out/sitemap.xml", b.sitemap.Marshal(), 0644)
	if err != nil {
		panic(err)
	}
	return
}

func (b *Builder) buildPostHTML(posts <-chan Post) <-chan Post {
	outCh := make(chan Post, 10)
	go func() {
		defer close(outCh)
		b.setupWaitGroup.Wait()
		for post := range posts {
			var doc bytes.Buffer
			err := b.postTemplate.Execute(&doc, post)
			if err != nil {
				panic(err)
			}
			post.HTML = doc.Bytes()
			outCh <- post // Forward the post to outCh
		}
	}()
	return outCh
}

func (b *Builder) writePostOut(posts <-chan Post) {
	var wg sync.WaitGroup
	for post := range posts {
		wg.Add(2)
		go func() {
			defer wg.Done()
			err := os.WriteFile(fmt.Sprintf("./out/post/%s.html", post.OGName), post.HTML, 0644)
			if err != nil {
				panic(err)
			}
		}()
		go func() {
			defer wg.Done()
			GenerateOG(post.Title, fmt.Sprintf("./out/og_images/%s.png", post.OGName), b.Config.OGImageConfig)
		}()
	}
	wg.Wait()
}

func (b *Builder) getFuncsMap() template.FuncMap {
	return template.FuncMap{
		"TagToURL": func(inp string) string {
			out := strings.ToLower(inp)
			out = strings.ReplaceAll(out, " ", "-")
			return out
		},
	}
}

func (b *Builder) setupHTML(inputDirectory string) {
	b.postTemplate = template.Must(template.New("post.html").Funcs(b.getFuncsMap()).ParseFiles(fmt.Sprintf("%s/templates/post.html", inputDirectory)))
	b.indexTemplate = template.Must(template.New("index.html").Funcs(b.getFuncsMap()).ParseFiles(fmt.Sprintf("%s/templates/index.html", inputDirectory)))
	b.tagTemplate = template.Must(template.New("tag.html").Funcs(b.getFuncsMap()).ParseFiles(fmt.Sprintf("%s/templates/tag.html", inputDirectory)))

	b.setupWaitGroup.Done()
}
