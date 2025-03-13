package builder

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	ToC        template.HTML
}

type PostMetadata struct {
	Slug    string
	Title   string
	Date    string
	Summary string
	Author  string
}

type Builder struct {
	MaxConcurrentPageBuilds int
	Config                  Config
	setupWaitGroup          sync.WaitGroup // setup things like parsing index.html, page.html

	indexCreated sync.WaitGroup

	postTemplate  *template.Template
	indexTemplate *template.Template
}

func (b *Builder) Build() {
	now := time.Now()
	b.setupWaitGroup.Add(3)
	b.indexCreated.Add(1)
	go b.setupPostHTML(b.Config.InputDirectory)
	go b.setupIndexHTML(b.Config.InputDirectory)
	go b.setupOutDirectory()

	postsChan, metadataChan := b.scanForMarkdownFiles(b.Config.InputDirectory)
	go b.buildIndexHTML(metadataChan)
	doneCh := b.buildPostHTML(postsChan)

	b.writePostOut(doneCh)

	b.indexCreated.Wait()

	took := time.Now().Sub(now)

	fmt.Println("All done!", took)
}
func (b *Builder) setupOutDirectory() {
	defer b.setupWaitGroup.Done()
	outDir := "out"
	scaffoldDirs := []string{"og_images", "assets", "post"}

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

func (b *Builder) buildIndexHTML(metadata <-chan PostMetadata) {
	out := []PostMetadata{}
	for meta := range metadata {
		out = append(out, meta)
	}

	var doc bytes.Buffer
	err := b.indexTemplate.Execute(&doc, out)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("./out/index.html", doc.Bytes(), 0644)
	if err != nil {
		panic(err)
	}

	b.indexCreated.Done()
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

func (b *Builder) setupPostHTML(inputDirectory string) {
	b.postTemplate = template.Must(template.ParseFiles(fmt.Sprintf("%s/templates/post.html", inputDirectory)))
	b.setupWaitGroup.Done()
}

func (b *Builder) setupIndexHTML(inputDirectory string) {
	b.indexTemplate = template.Must(template.ParseFiles(fmt.Sprintf("%s/templates/index.html", inputDirectory)))
	b.setupWaitGroup.Done()
}
