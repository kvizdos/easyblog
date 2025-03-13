package main

import (
	"flag"

	"github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
	"github.com/kvizdos/easyblog/builder"
)

var configPath = flag.String("config", "config.yaml", "Specify a path to a config file")

func main() {
	flag.Parse()

	cfg := builder.Config{}
	jsonFeeder := feeder.Yaml{Path: *configPath}

	// Create a Config instance and feed `myConfig` using `jsonFeeder`
	c := config.New()
	c.AddFeeder(jsonFeeder)
	c.AddStruct(&cfg)
	err := c.Feed()
	if err != nil {
		panic(err)
	}

	build := builder.Builder{
		MaxConcurrentPageBuilds: 5,
		Config:                  cfg,
	}
	build.Build()
	// Create a new Goldmark instance with GitHub Flavored Markdown extension.
	// md := goldmark.New(
	// 	goldmark.WithExtensions(extension.GFM),
	// )

	// input := []byte("```go\nvar x = 10;\n```")
	// var buf bytes.Buffer

	// // Convert Markdown to HTML.
	// if err := md.Convert(input, &buf); err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Println(buf.String())
}
