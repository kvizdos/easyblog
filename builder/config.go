package builder

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type OGImageConfig struct {
	IconPath string  `yaml:"IconPath"`
	FontPath string  `yaml:"FontPath"`
	FontSize float64 `yaml:"FontSize"`
	BgR      int     `yaml:"BgR"`
	BgG      int     `yaml:"BgG"`
	BgB      int     `yaml:"BgB"`
	TextR    int     `yaml:"TextR"`
	TextG    int     `yaml:"TextG"`
	TextB    int     `yaml:"TextB"`
}

type StaticConfig struct {
	Path string `yaml:"Path"`
}

type FeedConfig struct {
	Enabled     bool `yaml:"Enabled"`
	Limit       int  `yaml:"Limit"`       // max items per feed, defaults to 20
	FullContent bool `yaml:"FullContent"` // include the full post body, not just the summary
}

type SitemapConfig struct {
	Exclude    []string `yaml:"Exclude"`    // paths to leave out, e.g. "/hello"
	GitLastMod bool     `yaml:"GitLastMod"` // use `git log` for static page lastmod instead of file mtime
}

const (
	SlugStyleFilename = "filename"
	SlugStyleSlugify  = "slugify"
)

type Config struct {
	InputDirectory  string        `yaml:"InputDirectory"`
	OutputDirectory string        `yaml:"OutputDirectory"` // defaults to "out"
	BaseURL         string        `yaml:"BaseURL"`
	Title           string        `yaml:"Title"`
	Description     string        `yaml:"Description"`
	Author          string        `yaml:"Author"` // default author for posts without one
	OGImageConfig   OGImageConfig `yaml:"OGImageConfig"`
	CodeStyle       string        `yaml:"CodeStyle"` // Chroma Style
	StaticConfig    StaticConfig  `yaml:"StaticConfig"`
	Feed            FeedConfig    `yaml:"Feed"`
	Sitemap         SitemapConfig `yaml:"Sitemap"`

	// SlugStyle is "filename" (default, /post/My-File) or "slugify" (/post/my-file).
	SlugStyle string `yaml:"SlugStyle"`
	// CleanURLs writes out/post/X/index.html instead of out/post/X.html.
	CleanURLs bool `yaml:"CleanURLs"`
	// SkipFuturePosts leaves out posts dated after the build (override with -future).
	SkipFuturePosts bool `yaml:"SkipFuturePosts"`
	// RelatedPosts is how many related posts to compute per post. 0 means 3, negative disables.
	RelatedPosts int `yaml:"RelatedPosts"`
	// ToCMinHeadings empties Post.ToC when a post has fewer headings than this.
	ToCMinHeadings int `yaml:"ToCMinHeadings"`
	// StrictAlt fails the build when an image has no alt text (otherwise it only warns).
	StrictAlt bool `yaml:"StrictAlt"`
}

// Validate checks the config and fills in defaults.
func (c *Config) Validate() error {
	var errs []error

	if c.BaseURL == "" {
		errs = append(errs, errors.New("config: BaseURL is required"))
	} else if strings.HasSuffix(c.BaseURL, "/") {
		errs = append(errs, fmt.Errorf("config: BaseURL %q must not end with a slash", c.BaseURL))
	} else if u, err := url.Parse(c.BaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, fmt.Errorf("config: BaseURL %q must be an absolute URL like https://example.com", c.BaseURL))
	}

	switch c.SlugStyle {
	case "":
		c.SlugStyle = SlugStyleFilename
	case SlugStyleFilename, SlugStyleSlugify:
	default:
		errs = append(errs, fmt.Errorf("config: SlugStyle must be %q or %q, got %q", SlugStyleFilename, SlugStyleSlugify, c.SlugStyle))
	}

	if c.InputDirectory == "" {
		c.InputDirectory = "."
	}
	if c.OutputDirectory == "" {
		c.OutputDirectory = "out"
	}
	if c.Feed.Limit <= 0 {
		c.Feed.Limit = 20
	}
	if c.RelatedPosts == 0 {
		c.RelatedPosts = 3
	}

	return errors.Join(errs...)
}
