package builder

import (
	"reflect"
	"testing"
	"time"
)

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Bottlenecked-by-My-SSD-—-Not-My-Code:-Searching-Roughly-3M-JSON-Records-in-130ms": "bottlenecked-by-my-ssd-not-my-code-searching-roughly-3m-json-records-in-130ms",
		"One-File,-No-Merge": "one-file-no-merge",
		"Thinking Out Loud":  "thinking-out-loud",
		"C++ & Go":           "c-go",
		"Café Déjà Vu":       "cafe-deja-vu",
		"  --Go--  ":         "go",
	} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseDate(t *testing.T) {
	want := time.Date(2025, 4, 21, 0, 0, 0, 0, time.UTC)
	for _, in := range []any{"04/21/2025", "2025-04-21", " 2025-04-21 ", want} {
		got, err := parseDate(in)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseDate(%v) = %v, %v", in, got, err)
		}
	}
	for _, in := range []any{"21/04/2025", "", 42} {
		if _, err := parseDate(in); err == nil {
			t.Errorf("parseDate(%v) should fail", in)
		}
	}
}

func TestStringList(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want []string
	}{
		{"Go, JavaScript", []string{"Go", "JavaScript"}},
		{"Go,JavaScript", []string{"Go", "JavaScript"}},
		{"Go,  JS, ", []string{"Go", "JS"}},
		{[]any{"Go", " JS "}, []string{"Go", "JS"}},
	} {
		got, err := stringList(tc.in)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("stringList(%v) = %v, %v; want %v", tc.in, got, err, tc.want)
		}
	}
}

func TestCountWordsSkipsFrontmatterAndCode(t *testing.T) {
	src := "---\nTitle: lots of words here\n---\n\n## Two words\n\n```go\nignored code here\n```\n\nthree more words - \n"
	if got := countWords([]byte(src)); got != 5 {
		t.Errorf("countWords = %d, want 5", got)
	}
}

func TestRelatedPosts(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2025, 1, d, 0, 0, 0, 0, time.UTC) }
	all := PostList{
		{Slug: "/a", Tags: []string{"Go"}, PublishedAt: day(5)},
		{Slug: "/b", Tags: []string{"Go", "Docker"}, PublishedAt: day(4)},
		{Slug: "/c", Tags: []string{"Docker"}, PublishedAt: day(3)},
		{Slug: "/d", Tags: []string{"Rust"}, PublishedAt: day(2)},
		{Slug: "/self", Tags: []string{"Go", "Docker"}, PublishedAt: day(1)},
	}
	var got []string
	for _, p := range relatedPosts(all[4], all, 2) {
		got = append(got, p.Slug)
	}
	if want := []string{"/b", "/a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("related = %v, want %v", got, want)
	}
}

func TestConfigValidate(t *testing.T) {
	for _, base := range []string{"", "https://kv.codes/", "kv.codes"} {
		c := Config{BaseURL: base}
		if err := c.Validate(); err == nil {
			t.Errorf("BaseURL %q should be rejected", base)
		}
	}
	c := Config{BaseURL: "https://kv.codes"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.OutputDirectory != "out" || c.SlugStyle != SlugStyleFilename || c.Feed.Limit != 20 || c.RelatedPosts != 3 {
		t.Errorf("defaults not applied: %+v", c)
	}
}
