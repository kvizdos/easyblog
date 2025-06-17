package entrypoint

import (
	"flag"
	"html/template"

	"github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
	"github.com/kvizdos/easyblog/builder"
	"github.com/kvizdos/easyblog/quickstart"
)

var configPath = flag.String("config", "config.yaml", "Specify a path to a config file")

var quickStart = flag.Bool("quickstart", false, "Set to true to scaffold out your project")

var quickStartTargetDir = flag.String("target", ".", "Quick start target output directory")

var serve = flag.Bool("serve", false, "Set to true to serve your project FOR DEVELOPMENT.")

var servePort = flag.String("port", "8080", "Change the default port of the Serve")

type EasyblogOpts struct {
	CustomFuncs       template.FuncMap
	CustomOGGenerator builder.OGGeneratorFunc
}

func Start(opts EasyblogOpts) {
	flag.Parse()

	if *quickStart {
		quickstart.Scaffold(*quickStartTargetDir)
		return
	}

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
		CustomFuncs:             opts.CustomFuncs,
		OGGenerator:             opts.CustomOGGenerator,
	}

	if *serve == true {
		build.Serve(*servePort)
		return
	}

	build.Build()
}
