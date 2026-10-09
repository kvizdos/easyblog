# EasyBlog

Just make a GitHub Repo, and your blogging.

## Features

- [x] Markdown Support for Blog Posts
- [x] Automatic "index.html" creation w/ a list of all blog posts
- [x] OG Image Creation (cached between rebuilds in `-serve`)
- [x] Support Tags & have "Tag Pages"
- [x] Sitemap.xml Generation (posts, tags, `/`, and static pages, with `<lastmod>`)
- [x] robots.txt Generation
- [x] RSS 2.0 + Atom feeds, plus per-tag feeds
- [x] One-Off, Static Page Support (optionally templated)
- [x] Shared template partials
- [x] Drafts, scheduled posts, custom slugs, and redirects from old URLs
- [x] Related posts, prev/next, reading time
- [x] Run in `serve` mode for development.
  - [ ] TODO: Make this a bit more efficient; currently, it rebuilds the entire project on save. It seems unnecessary to do so.

## Usage

First, install EasyBlog:

```
$ go install github.com/kvizdos/easyblog
```

Now, scaffold your project with:

```
$ easyblog --quickstart
```

This will setup the project directory + create a GitHub Actions workflow to deploy to GH Pages

From there, customize HTML pages (in ./templates), add some styling, and add a post! You are off to the races.

If you'd like to build locally, run:

```
$ easyblog
```

This will build the files to `./out`. If the build fails, every problem (e.g. a post missing `Summary`) is listed and the exit code is non-zero.

You may also "serve" the files locally with the following command. This should only be used for development:

```
$ easyblog --serve --port 8080
```

(port is optional)

Other flags: `-drafts` builds `Draft: true` posts, `-future` builds future-dated posts when `SkipFuturePosts` is on, and `-version` prints the easyblog version.

## Configuration

```yaml
InputDirectory: .
OutputDirectory: out          # default "out"
BaseURL: https://example.com  # required, no trailing slash
Title: My Blog                # used by `seo`, feeds, and `site.Title`
Description: Notes on things
Author: Your Name             # default for posts without Author
CodeStyle: dracula            # https://github.com/alecthomas/chroma/tree/master/styles
StaticConfig:
  Path: ./static
Feed:
  Enabled: true               # writes rss.xml, atom.xml, tags/<tag>.xml
  Limit: 20
  FullContent: true           # full post body, with relative URLs made absolute
Sitemap:
  Exclude: [/hello]
  GitLastMod: false           # use git commit dates for static page <lastmod>
SlugStyle: filename           # or "slugify": /post/My-Post -> /post/my-post
CleanURLs: false              # write post/X/index.html (served at /post/X/)
SkipFuturePosts: false        # skip posts dated in the future unless -future
RelatedPosts: 3               # negative disables
ToCMinHeadings: 0             # hide the ToC on posts with fewer headings
StrictAlt: false              # fail the build on images without alt text
```

## Post frontmatter

```yaml
---
Title: My Post                  # defaults to the file name with - as spaces
Date: 04/21/2025                # or 2025-04-21; required unless Draft
Updated: 2025-05-01             # optional; used for dateModified, sitemap, feeds
Summary: One line about it.     # required
Author: Your Name               # required unless Author is set in config
Tags: Go, JavaScript            # or [Go, JavaScript]
Slug: my-post                   # optional: /post/my-post, or a full path like /notes/my-post
Aliases: [/post/Old-Name]       # old URLs that redirect here
Draft: true                     # only built with -drafts; never in index, tags, feeds, or sitemap
Syndications:
  bluesky: https://bsky.app/...
---
```

## Templates

- `templates/index.html` gets a `PostList`, `tag.html` gets `.Tag`, `.Posts`, `.Slug`, `.URL`, `.FeedURL`, and `post.html` gets a `Post`.
- Posts have `.URL` (absolute), `.PublishedAt` / `.UpdatedAt` (`time.Time`), `.WordCount`, `.ReadingMinutes`, `.HeadingCount`, `.Prev`, `.Next`, `.Related`, and `.Site`.
- `{{ site }}` returns the site from any template: `.BaseURL`, `.Title`, `.Description`, `.Author`, `.BuildTime`, `.Posts`, `.Tags`, `.FeedURL`, `.AtomURL`.
- Files in `templates/partials/*.html` are parsed into every template, so you can `{{ define "head" }}` once and `{{ template "head" . }}` everywhere.
- Files in `static/` ending in `.tmpl.html` are executed as templates (with partials) and written without `.tmpl`. They get a `Page` with `.Title`, `.Slug`, `.URL`, and `.Site`.
- Template funcs: `seo` (title, description, canonical, OpenGraph, Twitter, and feed links for whatever `.` is), `isoDate`, `rfc3339`, `rfc1123`, `formatDate "Jan 2, 2006"`, `absURL`, `slugify` / `TagToURL`, `contains`.

## See it in Action

Check out my personal dev blog here. It uses EasyBlog!

[https://github.com/kvizdos/kvizdos.github.io](https://github.com/kvizdos/kvizdos.github.io)
