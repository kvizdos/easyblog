package builder

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the current output")

// copyExample copies ../example into a temp dir with fixed mtimes, so
// sitemap lastmod values are stable.
func copyExample(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := copyDir("../example", dir); err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, fixed, fixed)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// fakeOG writes the title instead of rendering a PNG.
func fakeOG(calls *int) OGGeneratorFunc {
	return func(title, outPath string, _ OGImageConfig) {
		*calls++
		os.WriteFile(outPath, []byte(title), 0644)
	}
}

func newTestBuilder(dir string, ogCalls *int) *Builder {
	return &Builder{
		MaxConcurrentPageBuilds: 1, // fakeOG's counter isn't synchronized
		OGGenerator:             fakeOG(ogCalls),
		Config: Config{
			InputDirectory:  dir,
			OutputDirectory: filepath.Join(dir, "out"),
			BaseURL:         "https://example.com",
			Title:           "My Awesome Blog",
			Description:     "Notes on building things.",
			Author:          "Your Name",
			CodeStyle:       "dracula",
			StaticConfig:    StaticConfig{Path: "./static"},
			Feed:            FeedConfig{Enabled: true, FullContent: true},
			SkipFuturePosts: true,
			ToCMinHeadings:  2,
		},
	}
}

func readTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		data, err := os.ReadFile(path)
		files[filepath.ToSlash(rel)] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestGoldenExampleSite(t *testing.T) {
	dir := copyExample(t)
	var ogCalls int
	b := newTestBuilder(dir, &ogCalls)
	if err := b.Build(); err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "golden")
	out := filepath.Join(dir, "out")
	if *update {
		os.RemoveAll(golden)
		if err := copyDir(out, golden); err != nil {
			t.Fatal(err)
		}
		return
	}

	got, want := readTree(t, out), readTree(t, golden)
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("missing %s", name)
			continue
		}
		if !bytes.Equal(g, w) {
			t.Errorf("%s differs from golden (run go test ./builder -update to accept):\n%s", name, firstDiff(string(w), string(g)))
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("unexpected file %s", name)
		}
	}
}

func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return "line " + strconv.Itoa(i+1) + ":\n  want: " + wl + "\n  got:  " + gl
		}
	}
	return ""
}

func TestRebuildCleansAndCachesOG(t *testing.T) {
	dir := copyExample(t)
	var ogCalls int
	b := newTestBuilder(dir, &ogCalls)
	if err := b.Build(); err != nil {
		t.Fatal(err)
	}
	if ogCalls != 2 {
		t.Fatalf("expected 2 OG renders, got %d", ogCalls)
	}

	// Rename a post: the old page and OG image must disappear, and the
	// unchanged post's OG image must not be re-rendered.
	posts := filepath.Join(dir, "posts")
	if err := os.Rename(filepath.Join(posts, "second-post.md"), filepath.Join(posts, "renamed.md")); err != nil {
		t.Fatal(err)
	}
	if err := b.Build(); err != nil {
		t.Fatal(err)
	}
	if ogCalls != 3 {
		t.Errorf("expected only the renamed post's OG image to render, got %d total renders", ogCalls)
	}
	for _, stale := range []string{"out/post/second-post.html", "out/og_images/second-post.png"} {
		if _, err := os.Stat(filepath.Join(dir, stale)); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", stale)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "out/post/renamed.html")); err != nil {
		t.Error(err)
	}
}

func TestNonMarkdownFilesDoNotDeadlock(t *testing.T) {
	dir := copyExample(t)
	posts := filepath.Join(dir, "posts")
	for _, name := range []string{".DS_Store", "notes.txt", "a.txt", "b.txt"} {
		os.WriteFile(filepath.Join(posts, name), []byte("x"), 0644)
	}
	os.Mkdir(filepath.Join(posts, "images"), 0755)

	var ogCalls int
	b := newTestBuilder(dir, &ogCalls)
	done := make(chan error)
	go func() { done <- b.Build() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("build hung")
	}
}

func TestBadFrontmatterReportsEveryFile(t *testing.T) {
	dir := copyExample(t)
	posts := filepath.Join(dir, "posts")
	os.WriteFile(filepath.Join(posts, "no-summary.md"), []byte("---\nDate: 01/02/2025\nAuthor: A\n---\nhi\n"), 0644)
	os.WriteFile(filepath.Join(posts, "bad-types.md"), []byte("---\nDate: nope\nSummary: 42\nAuthor: A\n---\nhi\n"), 0644)

	var ogCalls int
	err := newTestBuilder(dir, &ogCalls).Build()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{
		"posts/no-summary.md: missing Summary",
		"posts/bad-types.md: Date:",
		"posts/bad-types.md: Summary must be a string",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestDraftsAndFuturePosts(t *testing.T) {
	dir := copyExample(t)
	future := time.Now().AddDate(1, 0, 0).Format("01/02/2006")
	os.WriteFile(filepath.Join(dir, "posts", "later.md"), []byte("---\nDate: "+future+"\nSummary: s\nTags: Go\n---\nhi\n"), 0644)

	var ogCalls int
	b := newTestBuilder(dir, &ogCalls)
	if err := b.Build(); err != nil {
		t.Fatal(err)
	}
	for _, missing := range []string{"out/post/draft-post.html", "out/post/later.html"} {
		if _, err := os.Stat(filepath.Join(dir, missing)); !os.IsNotExist(err) {
			t.Errorf("%s should not be built by default", missing)
		}
	}

	b.IncludeDrafts, b.IncludeFuture = true, true
	if err := b.Build(); err != nil {
		t.Fatal(err)
	}
	files := readTree(t, filepath.Join(dir, "out"))
	if _, ok := files["post/draft-post.html"]; !ok {
		t.Error("draft should be built with IncludeDrafts")
	}
	if _, ok := files["post/later.html"]; !ok {
		t.Error("future post should be built with IncludeFuture")
	}
	for _, name := range []string{"sitemap.xml", "rss.xml", "tags/example-tag-1.html", "index.html"} {
		if bytes.Contains(files[name], []byte("draft-post")) {
			t.Errorf("draft leaked into %s", name)
		}
	}
}

func TestCleanURLsAndSlugify(t *testing.T) {
	dir := copyExample(t)
	var ogCalls int
	b := newTestBuilder(dir, &ogCalls)
	b.Config.CleanURLs = true
	b.Config.SlugStyle = SlugStyleSlugify
	if err := b.Build(); err != nil {
		t.Fatal(err)
	}
	files := readTree(t, filepath.Join(dir, "out"))
	for _, want := range []string{
		"post/an-example-post/index.html",
		"post/old-second-post/index.html", // alias
		"tags/go/index.html",
		"about/index.html",
		"og_images/an-example-post.png",
	} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s", want)
		}
	}
	if !bytes.Contains(files["sitemap.xml"], []byte("<loc>https://example.com/post/an-example-post/</loc>")) {
		t.Errorf("sitemap should use trailing-slash URLs:\n%s", files["sitemap.xml"])
	}
}

func TestRobotsTxtAppendsSitemap(t *testing.T) {
	dir := copyExample(t)
	os.WriteFile(filepath.Join(dir, "static", "robots.txt"), []byte("User-agent: *\nDisallow: /private"), 0644)
	var ogCalls int
	if err := newTestBuilder(dir, &ogCalls).Build(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "out", "robots.txt"))
	want := "User-agent: *\nDisallow: /private\n\nSitemap: https://example.com/sitemap.xml\n"
	if string(got) != want {
		t.Errorf("robots.txt = %q, want %q", got, want)
	}
}

func TestMissingStaticDirIsAnError(t *testing.T) {
	dir := copyExample(t)
	os.RemoveAll(filepath.Join(dir, "static"))
	var ogCalls int
	if err := newTestBuilder(dir, &ogCalls).Build(); err == nil || !strings.Contains(err.Error(), "static") {
		t.Errorf("expected a static files error, got %v", err)
	}
}
