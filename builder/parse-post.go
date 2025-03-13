package builder

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

func ParsePost(postsChan chan<- Post, inputDirectory string, fileName string) {
	postMd, err := os.ReadFile(fmt.Sprintf("%s/%s", inputDirectory, fileName))
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)

	var buf bytes.Buffer

	// Convert Markdown to HTML.
	if err := md.Convert(postMd, &buf); err != nil {
		log.Fatal(err)
	}

	strippedFileName := fileName[:len(fileName)-3]

	postsChan <- Post{
		Title:       strings.ReplaceAll(strippedFileName, "-", " "),
		Description: "",
		Path:        fmt.Sprintf("/post/%s", strippedFileName),
		Body:        template.HTML(buf.String()),
		OGName:      strippedFileName,
	}
}
