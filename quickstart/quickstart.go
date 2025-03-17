package quickstart

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	embedded_example "github.com/kvizdos/easyblog/example"
)

func extractFiles(embedFS embed.FS, baseDir, targetDir string) error {
	// Walk through the embedded files starting from `baseDir`
	return fs.WalkDir(embedFS, baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute the destination path correctly
		relPath, err := filepath.Rel(baseDir, path) // Get relative path
		if err != nil {
			return err
		}

		destPath := filepath.Join(targetDir, relPath)

		if d.IsDir() {
			// Create directory if it does not exist
			return os.MkdirAll(destPath, os.ModePerm)
		}

		// Read file content
		data, err := embedFS.ReadFile(path)
		if err != nil {
			return err
		}

		// Write file to disk
		return os.WriteFile(destPath, data, 0644)
	})
}

func Scaffold(targetDir string) error {
	fmt.Println("Scaffolding project...")

	// Ensure target directory exists
	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		return err
	}

	// Extract all embedded files
	dirs := map[string]embed.FS{
		"templates": embedded_example.TemplatesContent,
		"posts":     embedded_example.PostsContent,
		"og":        embedded_example.OgContent,
		"assets":    embedded_example.AssetsContent,
		".github":   embedded_example.GitHubContent,
	}

	// Iterate over directories and extract contents
	for dir, fsContent := range dirs {
		if err := extractFiles(fsContent, dir, filepath.Join(targetDir, dir)); err != nil {
			return err
		}
	}

	// Write config.yaml separately
	configPath := filepath.Join(targetDir, "config.yaml")
	if err := os.WriteFile(configPath, embedded_example.ConfigContent, 0644); err != nil {
		return err
	}

	fmt.Println("Scaffold complete!")
	return nil
}
