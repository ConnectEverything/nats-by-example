package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/russross/blackfriday/v2"

	_ "embed"
)

var (
	languageOrder = []string{
		CLI,
		Go,
		Python,
		Deno,
		Node,
		Rust,
		CSharp,
		Java,
		Ruby,
		Elixir,
		C,
	}
)

var (
	//go:embed tmpl/head.html
	headInclude string

	//go:embed tmpl/logo.html
	logoInclude string

	//go:embed tmpl/index.html
	indexPage string

	//go:embed tmpl/category.html
	categoryPage string

	//go:embed tmpl/example.html
	examplePage string

	//go:embed tmpl/client.html
	clientPage string
)

type LanguageLink struct {
	Name  string
	Label string
	Path  string
}

type Link struct {
	Label string
	Path  string
}

type indexCategory struct {
	Title        string
	Description  template.HTML
	ExampleLinks []*Link
}

type indexData struct {
	Categories []*indexCategory
}

type categoryData struct {
	Title       string
	Description template.HTML
	Examples    []*Link
}

type exampleData struct {
	CategoryTitle string
	CategoryPath  string
	Title         string
	Description   template.HTML
	Path          string
	Links         []*LanguageLink
}

type clientData struct {
	CategoryTitle      string
	CategoryPath       string
	ExampleTitle       string
	ExamplePath        string
	ExampleDescription template.HTML
	RunPath            string
	Path               string
	SourceURL          string
	AsciinemaURL       template.URL
	Language           string
	Links              []*LanguageLink
	Blocks             []*RenderedBlock
	Output             string
	JSEscaped          string
	CanonicalURL       template.URL
	CanonicalImageURL  template.URL
}

func generateDocs(root *Root, dir string) error {
	t := template.New("site")

	_, err := t.New("head").Parse(headInclude)
	if err != nil {
		return err
	}

	_, err = t.New("logo").Parse(logoInclude)
	if err != nil {
		return err
	}

	rt, err := t.New("index").Parse(indexPage)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}

	ct, err := t.New("category").Parse(categoryPage)
	if err != nil {
		return fmt.Errorf("category: %w", err)
	}

	et, err := t.New("example").Parse(examplePage)
	if err != nil {
		return fmt.Errorf("example: %w", err)
	}

	it, err := t.New("client").Parse(clientPage)
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}

	buf := bytes.NewBuffer(nil)

	var ics []*indexCategory
	for _, c := range root.Categories {
		ic := indexCategory{
			Title:       c.Title,
			Description: template.HTML(blackfriday.Run([]byte(c.Description))),
		}
		ics = append(ics, &ic)
		for _, e := range c.Examples {
			l := &Link{
				Label: e.Title,
			}
			for _, k := range languageOrder {
				if c, ok := e.Clients[k]; ok {
					l.Path = c.Path
					break
				}
			}
			// Use the example title, but with the first client path.
			ic.ExampleLinks = append(ic.ExampleLinks, l)
		}
	}

	ix := indexData{
		Categories: ics,
	}

	err = rt.Execute(buf, &ix)
	if err != nil {
		return err
	}

	err = createFile(filepath.Join(dir, "index.html"), buf.Bytes())
	if err != nil {
		return err
	}

	for _, c := range root.Categories {
		buf.Reset()

		var elinks []*Link
		for _, e := range c.Examples {
			elinks = append(elinks, &Link{
				Label: e.Title,
				Path:  e.Path,
			})
		}

		cx := categoryData{
			Title:       c.Title,
			Description: template.HTML(blackfriday.Run([]byte(c.Description))),
			Examples:    elinks,
		}
		err = ct.Execute(buf, &cx)
		if err != nil {
			return err
		}

		err = createFile(filepath.Join(dir, c.Path, "index.html"), buf.Bytes())
		if err != nil {
			return err
		}

		for _, e := range c.Examples {
			buf.Reset()

			links := make([]*LanguageLink, len(languageOrder))
			for i, n := range languageOrder {
				l := &LanguageLink{
					Name:  n,
					Label: availableLanguages[n],
				}
				for _, i := range e.Clients {
					if i.Language == n {
						l.Path = i.Path
						break
					}
				}
				links[i] = l
			}

			ex := exampleData{
				CategoryTitle: c.Title,
				CategoryPath:  c.Path,
				Description:   template.HTML(blackfriday.Run([]byte(e.Description))),
				Title:         e.Title,
				Path:          e.Path,
				Links:         links,
			}
			err = et.Execute(buf, &ex)
			if err != nil {
				return err
			}

			err = createFile(filepath.Join(dir, e.Path, "index.html"), buf.Bytes())
			if err != nil {
				return err
			}

			for _, i := range e.Clients {
				var rblocks []*RenderedBlock
				// Always start with a comment block...
				if i.Blocks[0].Type == CodeBlock {
					rblocks = append(rblocks, &RenderedBlock{Type: "comment"})
				}

				for _, b := range i.Blocks {
					rb, err := renderBlock(i.Language, b)
					if err != nil {
						return err
					}
					rblocks = append(rblocks, rb)
				}

				outputFile := filepath.Join(i.Path, "output.txt")

				var castFile string
				outputBytes, err := ioutil.ReadFile(outputFile)
				if err != nil {
					if !os.IsNotExist(err) {
						log.Printf("%s: %s", outputFile, err)
					} else {
						return err
					}
				} else {
					castFile = filepath.Join(i.Path, "output.cast")
				}

				ix := clientData{
					CategoryTitle:      c.Title,
					CategoryPath:       c.Path,
					ExampleTitle:       e.Title,
					ExamplePath:        e.Path,
					ExampleDescription: ex.Description,
					Path:               i.Path,
					RunPath:            strings.TrimPrefix(i.Path, "examples/"),
					SourceURL:          "https://github.com/bruth/nats-by-example/tree/main/" + i.Path,
					AsciinemaURL:       template.URL(castFile),
					Output:             string(outputBytes),
					Links:              links,
					Language:           availableLanguages[i.Language],
					Blocks:             rblocks,
					JSEscaped:          i.Source,
				}

				buf.Reset()
				err = it.Execute(buf, &ix)

				err = createFile(filepath.Join(dir, i.Path, "index.html"), buf.Bytes())
				if err != nil {
					return err
				}

				if err := copyFile(castFile, filepath.Join(dir, castFile)); err != nil {
					if os.IsNotExist(err) {
						log.Printf("%s: %s", castFile, err)
					} else {
						return err
					}
				}
			}
		}
	}

	return nil
}

func commonPrefixForLines(lines []string, delim string) (string, int) {
	// Find the first line with a prefix.
	for i, l := range lines {
		// Ignore leading empty lines.
		if strings.TrimSpace(l) == "" {
			continue
		}

		// Get the leading whitespace for the first comment line. Assume all are
		// indented at the same level.
		idx := strings.Index(l, delim)
		return l[:idx], i
	}

	return "", -1
}

// Remove leading whitespace and comment delimiters. Remove shortest whitespace
// prefix after delimiters. Leading and trailing empty lines are removed, interleaved
// ones are preserved for rendering.
func cleanSingleCommentLines(lines []string, delim string) (string, string) {
	prefix, first := commonPrefixForLines(lines, delim)

	var cleaned []string
	for _, l := range lines[first:] {
		// Trim leading whitespace.
		l = strings.TrimPrefix(l, prefix)
		// Trim comment characters.
		l = strings.TrimPrefix(l, delim)
		// Assume space after delim.
		l = strings.TrimPrefix(l, " ")
		cleaned = append(cleaned, l)
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n")), prefix
}

// Dedent relative to the smallest indent for all lines. Also remove
// the comment syntax.
func cleanMultiCommentLines(lines []string) (string, string) {
	prefix, first := commonPrefixForLines(lines, "/*")

	var cleaned []string
	var lastIdx int
	for i, l := range lines[first:] {
		l = strings.TrimPrefix(l, prefix)
		if i == 0 {
			l = strings.TrimPrefix(l, "/*")
		}
		cleaned = append(cleaned, l)
		if strings.TrimSpace(l) != "" {
			lastIdx = i
		}
	}

	last := cleaned[lastIdx]
	cleaned[lastIdx] = strings.TrimSuffix(last, "*/")

	return strings.TrimSpace(strings.Join(cleaned, "\n")), prefix
}

func chromaFormat(code, lang string) (string, error) {
	switch lang {
	case Shell, CLI:
		lang = "sh"
	case Deno, Bun:
		lang = "ts"
	case Node:
		lang = "js"
	case WebSocket:
		lang = "js"
	}

	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	if lang == "output" {
		lexer = SimpleShellOutputLexer
	}

	lexer = chroma.Coalesce(lexer)

	style := styles.Get("swapoff")
	if style == nil {
		style = styles.Fallback
	}
	formatter := html.New(html.WithClasses(true))
	iterator, err := lexer.Tokenise(nil, string(code))
	if err != nil {
		return "", err
	}
	buf := bytes.NewBuffer(nil)
	err = formatter.Format(buf, style, iterator)
	return buf.String(), nil
}

func renderBlock(lang string, block *Block) (*RenderedBlock, error) {
	var r RenderedBlock
	switch block.Type {
	case CodeBlock:
		r.Type = "code"
		text := strings.Join(block.Lines, "\n")
		html, err := chromaFormat(text, lang)
		if err != nil {
			return nil, err
		}
		r.HTML = template.HTML(html)

	case SingleLineCommentBlock:
		delim := languageLineCommentDelim[lang]
		text, indent := cleanSingleCommentLines(block.Lines, delim)
		r.Type = "comment"
		r.HTML = template.HTML(blackfriday.Run([]byte(text)))
		r.Prefix = indent

	case MultiLineCommentBlock:
		text, indent := cleanMultiCommentLines(block.Lines)
		r.Type = "comment"
		r.HTML = template.HTML(blackfriday.Run([]byte(text)))
		r.Prefix = indent
	}

	return &r, nil
}

type RenderedBlock struct {
	// Comment or Code
	Type string

	// HTML rendered content. comment -> markdown, code -> syntax highlighted
	HTML template.HTML

	// Prefix string for the non-empty lines.
	Prefix string
}

var SimpleShellOutputLexer = chroma.MustNewLexer(
	&chroma.Config{
		Name:      "Shell Output",
		Aliases:   []string{"console"},
		Filenames: []string{"*.sh"},
		MimeTypes: []string{},
	},
	chroma.Rules{
		"root": {
			// $ or > triggers the start of prompt formatting
			{`^\$`, chroma.GenericPrompt, chroma.Push("prompt")},
			{`^>`, chroma.GenericPrompt, chroma.Push("prompt")},

			// empty lines are just text
			{`^$\n`, chroma.Text, nil},

			// otherwise its all output
			{`[^\n]+$\n?`, chroma.GenericOutput, nil},
		},
		"prompt": {
			// when we find newline, do output formatting rules
			{`\n`, chroma.Text, chroma.Push("output")},
			// otherwise its all text
			{`[^\n]+$`, chroma.Text, nil},
		},
		"output": {
			// sometimes there isn't output so we go right back to prompt
			{`^\$`, chroma.GenericPrompt, chroma.Pop(1)},
			{`^>`, chroma.GenericPrompt, chroma.Pop(1)},
			// otherwise its all output
			{`[^\n]+$\n?`, chroma.GenericOutput, nil},
		},
	},
)
