package main

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/russross/blackfriday/v2"
)

var (
	languageOrder = []string{
		CLI,
		Go,
		Python,
		Deno,
		Rust,
		CSharp,
		Java,
		Ruby,
		Elixir,
		C,
	}
)

var (
	tocPage = `<!doctype html>
<html>
<head>
	<title>NATS by Example</title>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<link rel="icon" href="/nats.svg" />
	<link rel="stylesheet" href="/main.css">
</head>
<body>
	<h1>NATS by Example</h1>

	{{range .Categories}}
		<h3>{{.Title}}</h3>
		<ul>
		{{range .Examples}}
			<li><a href="/{{.Path}}">{{.Title}}</a></li>
		{{end}}
		</ul>
	{{end}}
</body>
</html>
`

	catPage = `<!doctype html>
<html>
<head>
	<title>NATS by Example</title>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<link rel="icon" href="/nats.svg" />
	<link rel="stylesheet" href="/main.css">
</head>
<body>
	<h1><a href="/">NATS by Example</a></h1>

	<h2 class="title">{{.Title}}</h2>

	<div class="description">
	{{.Description}}
	</div>

	<ul>
	{{range .Examples}}
		<li><a href="/{{.Path}}">{{.Label}}</a></li>
	{{end}}
	</ul>
</body>
</html>
`

	examplePage = `<!doctype html>
<html>
<head>
	<title>NATS by Example</title>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<link rel="icon" href="/nats.svg" />
	<link rel="stylesheet" href="/main.css">
</head>
<body>
	<h1><a href="/">NATS by Example</a></h1>

	<div><a href="/{{.CategoryPath}}">{{.CategoryTitle}}</a></div>
	<h2 class="title">{{.Title}}</h2>

	<div class="description">
	{{.Description}}
	</div>

	<div class="language-tabs">
	{{range .Links}}
	{{if .Path}}
	<span><a href="/{{.Path}}">{{.Label}}</a></span>
	{{else}}
	<span class="quiet" title="Not yet implemented">{{.Label}}</span>
	{{end}}
	{{end}}
	</div>

</body>
</html>
`

	implPage = `<!doctype html>
<html>
<head>
	<title>NATS by Example</title>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<link rel="icon" href="/nats.svg" />
	<link rel="stylesheet" href="/main.css">
</head>
<body>
	<h1><a href="/">NATS by Example</a></h1>

	<div><a href="/{{.CategoryPath}}">{{.CategoryTitle}}</a></div>
	<h2 class="title">{{.ExampleTitle}}</h2>

	<div class="description">
	{{.ExampleDescription}}
	</div>

	<div class="info">
		<div class="language-tabs">
		{{range .Links}}
		{{if .Path}}
		<span><a href="/{{.Path}}">{{.Label}}</a></span>
		{{else}}
		<span class="quiet" title="Not yet implemented">{{.Label}}</span>
		{{end}}
		{{end}}
		</div>

		<div class="source-run">
			Run this example using
			<a href="https://github.com/bruth/nats-by-example/releases"><code>nbe</code></a>
			<pre>nbe run {{.RunPath}}</pre>
			<small><a href="{{.SourceURL}}" target=_blank>View source code</a></small>
		</div>
	</div>

	<div class="example">
	{{range .Blocks}}
	{{if eq .Type "comment" }}
		<div class="example-comment">
		{{.HTML}}
		</div>
	{{else}}
		<div class="example-code">
		{{.HTML}}
		</div>
	{{end}}
	{{end}}
	</div>
</body>
</html>
`
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

type categoryTmplData struct {
	Title       string
	Description template.HTML
	Examples    []*Link
}

type exampleTmplData struct {
	CategoryTitle string
	CategoryPath  string
	Title         string
	Description   template.HTML
	Path          string
	Links         []*LanguageLink
}

type implementationTmplData struct {
	CategoryTitle      string
	CategoryPath       string
	ExampleTitle       string
	ExamplePath        string
	ExampleDescription template.HTML
	RunPath            string
	Path               string
	SourceURL          string
	Language           string
	Links              []*LanguageLink
	Blocks             []*RenderedBlock
}

func generateDocs(root *Root, output string) error {
	rt, err := template.New("index").Parse(tocPage)
	if err != nil {
		return fmt.Errorf("index: %w", err)
	}

	ct, err := template.New("category").Parse(catPage)
	if err != nil {
		return fmt.Errorf("category: %w", err)
	}

	et, err := template.New("example").Parse(examplePage)
	if err != nil {
		return fmt.Errorf("example: %w", err)
	}

	it, err := template.New("impl").Parse(implPage)
	if err != nil {
		return fmt.Errorf("implementation: %w", err)
	}

	buf := bytes.NewBuffer(nil)

	err = rt.Execute(buf, root)
	if err != nil {
		return err
	}

	err = createFile(filepath.Join(output, "index.html"), buf.Bytes())
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

		cx := categoryTmplData{
			Title:       c.Title,
			Description: template.HTML(blackfriday.Run([]byte(c.Description))),
			Examples:    elinks,
		}
		err = ct.Execute(buf, &cx)
		if err != nil {
			return err
		}

		err = createFile(filepath.Join(output, c.Path, "index.html"), buf.Bytes())
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
				for _, i := range e.Implementations {
					if i.Language == n {
						l.Path = i.Path
						break
					}
				}
				links[i] = l
			}

			ex := exampleTmplData{
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

			err = createFile(filepath.Join(output, e.Path, "index.html"), buf.Bytes())
			if err != nil {
				return err
			}

			for _, i := range e.Implementations {
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

				ix := implementationTmplData{
					CategoryTitle:      c.Title,
					CategoryPath:       c.Path,
					ExampleTitle:       e.Title,
					ExamplePath:        e.Path,
					ExampleDescription: ex.Description,
					Path:               i.Path,
					RunPath:            strings.TrimPrefix(i.Path, "examples/"),
					SourceURL:          "https://github.com/bruth/nats-by-example/tree/main/" + i.Path,
					Links:              links,
					Language:           i.Language,
					Blocks:             rblocks,
				}

				buf.Reset()
				err = it.Execute(buf, &ix)

				err = createFile(filepath.Join(output, i.Path, "index.html"), buf.Bytes())
				if err != nil {
					return err
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
	case Deno, Node:
		lang = "ts"
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
