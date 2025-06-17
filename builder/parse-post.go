package builder

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"strings"

	figure "github.com/mangoumbrella/goldmark-figure"
	fences "github.com/stefanfritsch/goldmark-fences"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/anchor"
	"go.abhg.dev/goldmark/toc"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
)

type customTexter struct{}

func (*customTexter) AnchorText(h *anchor.HeaderInfo) []byte {
	if h.Level == 1 {
		return nil
	}
	return []byte("#")
}

func ParsePost(postsChan chan<- Post, metadataChan chan<- PostMetadata, config Config, fileName string) {
	postMd, err := os.ReadFile(fmt.Sprintf("%s/posts/%s", config.InputDirectory, fileName))
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}

	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // read note
		),
		goldmark.WithExtensions(extension.GFM, meta.Meta, figure.Figure, &anchor.Extender{
			Attributer: anchor.Attributes{
				"class": "headerPermalink",
			},
			Texter: &customTexter{},
		},
			&fences.Extender{},
			highlighting.NewHighlighting(
				highlighting.WithStyle(config.CodeStyle),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(true),
				),
			)),
	)

	var buf bytes.Buffer

	src := text.NewReader(postMd)
	doc := md.Parser().Parse(src)
	tree, err := toc.Inspect(doc, postMd, toc.MinDepth(2), toc.MaxDepth(3))
	if err != nil {
		panic(err)
		// handle the error
	}

	list := toc.RenderList(tree)

	toc := template.HTML("")
	if list != nil {
		var tocBuff bytes.Buffer
		err = md.Renderer().Render(&tocBuff, []byte{}, list)

		if err != nil {
			panic(err)
		}
		toc = template.HTML(tocBuff.String())
	}
	context := parser.NewContext()
	if err := md.Convert(postMd, &buf, parser.WithContext(context)); err != nil {
		panic(err)
	}

	metaData := meta.Get(context)

	strippedFileName := fileName[:len(fileName)-3]

	tags := []string{}
	if v, ok := metaData["Tags"].(string); ok {
		tags = strings.Split(v, ", ")
	}

	title := strings.ReplaceAll(strippedFileName, "-", " ")

	if v, ok := metaData["Title"].(string); ok {
		title = v
	}

	postsChan <- Post{
		Title:       title,
		Body:        template.HTML(buf.String()),
		OGName:      strippedFileName,
		Date:        metaData["Date"].(string),
		Author:      metaData["Author"].(string),
		Summary:     metaData["Summary"].(string),
		Tags:        tags,
		ToC:         toc,
		OGImageURL:  fmt.Sprintf("%s/og_images/%s.png", config.BaseURL, strippedFileName),
		RawMetadata: metaData,
	}

	metadataChan <- PostMetadata{
		RawMetadata: metaData,
		Slug:        fmt.Sprintf("/post/%s", strippedFileName),
		Title:       title,
		Date:        metaData["Date"].(string),
		Summary:     metaData["Summary"].(string),
		Author:      metaData["Author"].(string),
		Tags:        tags,
	}
}
