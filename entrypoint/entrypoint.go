package entrypoint

import (
	"flag"
	"fmt"
	"html/template"
	"os"
	"runtime/debug"

	"github.com/golobby/config/v3"
	"github.com/golobby/config/v3/pkg/feeder"
	"github.com/kvizdos/easyblog/builder"
	"github.com/kvizdos/easyblog/quickstart"
)

const modulePath = "github.com/kvizdos/easyblog"

var configPath = flag.String("config", "config.yaml", "Specify a path to a config file")

var quickStart = flag.Bool("quickstart", false, "Set to true to scaffold out your project")

var quickStartTargetDir = flag.String("target", ".", "Quick start target output directory")

var serve = flag.Bool("serve", false, "Set to true to serve your project FOR DEVELOPMENT.")

var servePort = flag.String("port", "8080", "Change the default port of the Serve")

var drafts = flag.Bool("drafts", false, "Build posts marked Draft: true")

var future = flag.Bool("future", false, "Build future-dated posts even when SkipFuturePosts is set")

var version = flag.Bool("version", false, "Print the easyblog version and exit")

type EasyblogOpts struct {
	CustomFuncs       template.FuncMap
	CustomOGGenerator builder.OGGeneratorFunc
}

// Version reports the easyblog module version, whether it's the main module
// (go install github.com/kvizdos/easyblog@vX) or a dependency of a custom entrypoint.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	if info.Main.Path == modulePath {
		return info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path == modulePath {
			if dep.Replace != nil {
				return dep.Version + " (replaced by " + dep.Replace.Path + ")"
			}
			return dep.Version
		}
	}
	return "unknown"
}

func Start(opts EasyblogOpts) {
	flag.Parse()

	if *version {
		fmt.Println("easyblog", Version())
		return
	}

	if *quickStart {
		if err := quickstart.Scaffold(*quickStartTargetDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	cfg := builder.Config{}
	jsonFeeder := feeder.Yaml{Path: *configPath}

	// Create a Config instance and feed `myConfig` using `jsonFeeder`
	c := config.New()
	c.AddFeeder(jsonFeeder)
	c.AddStruct(&cfg)
	if err := c.Feed(); err != nil {
		fmt.Fprintln(os.Stderr, "reading config:", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	build := builder.Builder{
		MaxConcurrentPageBuilds: 5,
		Config:                  cfg,
		CustomFuncs:             opts.CustomFuncs,
		OGGenerator:             opts.CustomOGGenerator,
		IncludeDrafts:           *drafts,
		IncludeFuture:           *future,
	}

	if *serve {
		build.Serve(*servePort)
		return
	}

	if err := build.Build(); err != nil {
		fmt.Fprintln(os.Stderr, "build failed:")
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
