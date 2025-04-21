# EasyBlog

Just make a GitHub Repo, and your blogging.

## Features

- [x] Markdown Support for Blog Posts
- [x] Automatic "index.html" creation w/ a list of all blog posts
- [x] OG Image Creation
- [x] Support Tags & have "Tag Pages"
- [x] Sitemap.xml Generation
- [x] One-Off, Static Page Support
- [x] Run in `serve` mode for development.
  - [ ] TODO: Make this a bit more efficient; currently, it rebuilds the entire project on save. It seems unnecessary to do so.
- TODO: RSS Feed

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

This will build the files to `./out`. Do note: when clicking into a page, add `.html` to the URL.

You may also "serve" the files locally with the following command. This should only be used for development:

```
$ easyblog --serve --port 8080
```

(port is optional)

## See it in Action

Check out my personal dev blog here. It uses EasyBlog!

[https://github.com/kvizdos/kvizdos.github.io](https://github.com/kvizdos/kvizdos.github.io)
