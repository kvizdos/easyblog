package embedded_example

import "embed"

//go:embed templates/*
var TemplatesContent embed.FS

//go:embed posts/*
var PostsContent embed.FS

//go:embed og/*
var OgContent embed.FS

//go:embed assets/*
var AssetsContent embed.FS

//go:embed .github/*
var GitHubContent embed.FS

//go:embed config.yaml
var ConfigContent []byte
