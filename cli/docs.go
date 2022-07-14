package main

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/russross/blackfriday/v2"
)

var (
	tocPage = `<!doctype html>
<html>
<head>
	<link rel="stylesheet" href="/main.css">
</head>
<body>
	<h1>NATS by Example</h1>

	{{range .Categories}}
		<h3>{{.Title}}</h3>
		<ul>
		{{range .Examples}}
			<li><a href="/{{.Path}}">{{.Title}}</a> - {{range .Implementations}}<a href="/{{.Path}}">{{.Name}}</a> {{end}}</li>
		{{end}}
		</ul>
	{{end}}
</body>
</html>
`

	catPage = `<!doctype html>
<html>
<head>
	<link rel="stylesheet" href="/main.css">
</head>
<body>
	<h1><a href="/">NATS by Example</a></h1>

	<h2>{{.Title}}</h2>

	{{range .Examples}}
	<h3><a href="/{{.Path}}">{{.Title}}</a></h3>
	<ul>
	{{range .Implementations}}
	<li><a href="/{{.Path}}">{{.Name}}</a></li>
	{{end}}
	</ul>
	{{end}}
</body>
</html>
`

	examplePage = `<!doctype html>
<html>
<head>
	<link rel="stylesheet" href="/main.css">
</head>
<body>
	<h1><a href="/">NATS by Example</a></h1>

	<h2>{{.Title}}</h2>

	<ul>
	{{range .Implementations}}
	<li><a href="/{{.Path}}">{{.Name}}</a></li>
	{{end}}
	</ul>
</body>
</html>
`

	implPage = `<!doctype html>
<html>
<head>
	<link rel="stylesheet" href="/main.css">
</head>
<body>
	<h1><a href="/">NATS by Example</a></h1>

	<div><a href="/{{.CategoryPath}}">{{.CategoryTitle}}</a> / <a href="/{{.ExamplePath}}">{{.ExampleTitle}}</a></div>
	<h2>{{.Language}}</h2>

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
</body>
</html>
`
)

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
		err = ct.Execute(buf, c)

		err = createFile(filepath.Join(output, c.Path, "index.html"), buf.Bytes())
		if err != nil {
			return err
		}

		for _, e := range c.Examples {
			buf.Reset()
			err = et.Execute(buf, e)

			err = createFile(filepath.Join(output, e.Path, "index.html"), buf.Bytes())
			if err != nil {
				return err
			}

			for _, i := range e.Implementations {
				rblocks := make([]*RenderedBlock, len(i.Blocks))
				for j, b := range i.Blocks {
					rb, err := renderBlock(i.Language, b)
					if err != nil {
						return err
					}
					rblocks[j] = rb
				}

				x := ImplPage{
					CategoryTitle: c.Title,
					CategoryPath:  c.Path,
					ExampleTitle:  e.Title,
					ExamplePath:   e.Path,
					Language:      i.Language,
					Blocks:        rblocks,
				}

				buf.Reset()
				err = it.Execute(buf, &x)

				err = createFile(filepath.Join(output, i.Path, "index.html"), buf.Bytes())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

var (
	wsRe = regexp.MustCompile(`^(\s+)`)
)

// Remove leading whitespace and comment delimiters. Remove shortest whitespace
// prefix after delimiters. Leading and trailing empty lines are removed, interleaved
// ones are preserved for rendering.
func cleanSingleCommentLines(lines []string) string {
	var cleaned []string

	offset := -1
	for _, l := range lines {
		// Remove the leading whitespace and comment characters.
		l := wsRe.ReplaceAllLiteralString(l, "")
		l = strings.TrimPrefix(l, "//")

		// Remove the shortest whitespace prefix after the comment since there may be
		m := wsRe.FindStringSubmatch(l)

		if len(m) == 0 {
			offset = 0
		} else {
			o := len(m[0])
			if offset == -1 || o < offset {
				offset = o
			}
		}

		cleaned = append(cleaned, l)
	}

	for i, l := range cleaned {
		if l == "" {
			continue
		}

		cleaned[i] = l[offset:]
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// Dedent relative to the smallest indent for all lines. Also remove
// the comment syntax.
func cleanMultiCommentLines(lines []string) string {
	offset := -1
	firstLine := -1

	var cleaned []string
	for i, l := range lines {
		if firstLine == -1 {
			// If there are lines with a smaller offset, these will be ignored. For
			// lines with a larger offset, preserve the indent.
			if idx := strings.Index(l, "/*"); idx >= 0 {
				l = l[idx+2:]
				firstLine = i
				cleaned = append(cleaned, l)
			}
		} else {
			cleaned = append(cleaned, l)
		}

		if strings.TrimSpace(l) != "" {
			// Remove the shortest whitespace prefix after the comment since there may be
			m := wsRe.FindStringSubmatch(l)
			if len(m) == 0 {
				offset = 0
			} else {
				o := len(m[0])
				if offset == -1 || o < offset {
					offset = o
				}
			}
		}
	}

	var lastIdx int
	for i, l := range cleaned {
		if strings.TrimSpace(l) == "" {
			continue
		}
		cleaned[i] = strings.TrimRight(l[offset:], " ")
		lastIdx = i
	}

	last := cleaned[lastIdx]
	index := strings.Index(last, "*/")
	cleaned[lastIdx] = last[:index]

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func cleanCodeLines(lines []string) string {
	offset := -1

	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}

		ms := wsRe.FindStringSubmatch(l)
		if len(ms) == 0 {
			offset = 0
		} else {
			o := len(ms[0])
			if offset == -1 || o < offset {
				offset = o
			}
		}
	}

	var cleaned []string
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		cleaned = append(cleaned, l[offset:])
	}

	return strings.Join(lines, "\n")

	return strings.Join(cleaned, "\n")
}

func chromaFormat(code, lang string) (string, error) {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	if lang == Shell {
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
		text := cleanCodeLines(block.Lines)
		html, err := chromaFormat(text, lang)
		if err != nil {
			return nil, err
		}
		r.HTML = template.HTML(html)
	case SingleLineCommentBlock:
		text := cleanSingleCommentLines(block.Lines)
		r.Type = "comment"
		r.HTML = template.HTML(blackfriday.Run([]byte(text)))
	case MultiLineCommentBlock:
		text := cleanMultiCommentLines(block.Lines)
		r.Type = "comment"
		r.HTML = template.HTML(blackfriday.Run([]byte(text)))
	}

	return &r, nil
}

type RenderedBlock struct {
	// Comment or Code
	Type string
	// HTML rendered content. comment -> markdown, code -> syntax highlighted
	HTML template.HTML
}

type ImplPage struct {
	CategoryTitle string
	CategoryPath  string
	ExampleTitle  string
	ExamplePath   string
	Language      string
	Blocks        []*RenderedBlock
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
