package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"gopkg.in/yaml.v3"

	"github.com/russross/blackfriday/v2"
)

// siteDir is the target directory into which the HTML gets generated. Its
// default is set here but can be changed by an argument passed into the
// program.
var siteDir = "./public"

func verbose() bool {
	return len(os.Getenv("VERBOSE")) > 0
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func isDir(path string) bool {
	fileStat, _ := os.Stat(path)
	return fileStat.IsDir()
}

func ensureDir(dir string) {
	err := os.MkdirAll(dir, 0755)
	check(err)
}

func copyFile(src, dst string) {
	dat, err := os.ReadFile(src)
	check(err)
	err = os.WriteFile(dst, dat, 0644)
	check(err)
}

func mustReadFile(path string) string {
	bytes, err := os.ReadFile(path)
	check(err)
	return string(bytes)
}

func markdown(src string) string {
	return string(blackfriday.Run([]byte(src)))
}

func readLines(path string) []string {
	src := mustReadFile(path)
	return strings.Split(src, "\n")
}

func mustGlob(glob string) []string {
	paths, err := filepath.Glob(glob)
	check(err)
	return paths
}

func whichLexer(path string) string {
	if strings.HasSuffix(path, ".go") {
		return "go"
	} else if strings.HasSuffix(path, ".sh") {
		return "console"
	}
	panic("No lexer for " + path)
}

func debug(msg string) {
	if os.Getenv("DEBUG") == "1" {
		fmt.Fprintln(os.Stderr, msg)
	}
}

var docsPat = regexp.MustCompile("^\\s*(\\/\\/|#)\\s")
var dashPat = regexp.MustCompile("\\-+")

type Category struct {
	ID          string
	Title string
	Description string
	ExampleList `yaml:"examples"`
	Examples []*Example `yaml:"-"`
}


// Segment is a segment of an example.
type Segment struct {
	Docs         string
	DocsRendered string
	Code         string
	CodeRendered string
	CodeForJs    string
	CodeEmpty    bool
	CodeLeading  bool
	CodeRun      bool
}

// Example is info extracted from an example file.
type Example struct {
	ID          string
	Name        string
	Category    string
	GoCode      string
	Segments    [][]*Segment
	PrevExample *Example
	NextExample *Example
}

func parseSegments(sourcePath string) ([]*Segment, string) {
	var (
		lines  []string
		source []string
	)

	// Convert tabs to spaces for uniform rendering.
	for _, line := range readLines(sourcePath) {
		lines = append(lines, strings.Replace(line, "\t", "    ", -1))
		source = append(source, line)
	}

	filecontent := strings.Join(source, "\n")
	segs := []*Segment{}
	lastSeen := ""

	for _, line := range lines {
		if line == "" {
			lastSeen = ""
			continue
		}

		matchDocs := docsPat.MatchString(line)
		matchCode := !matchDocs
		newDocs := (lastSeen == "") || ((lastSeen != "docs") && (segs[len(segs)-1].Docs != ""))
		newCode := (lastSeen == "") || ((lastSeen != "code") && (segs[len(segs)-1].Code != ""))
		if newDocs || newCode {
			debug("NEWSEG")
		}
		if matchDocs {
			trimmed := docsPat.ReplaceAllString(line, "")
			if newDocs {
				newSeg := Segment{Docs: trimmed, Code: ""}
				segs = append(segs, &newSeg)
			} else {
				segs[len(segs)-1].Docs = segs[len(segs)-1].Docs + "\n" + trimmed
			}
			debug("DOCS: " + line)
			lastSeen = "docs"
		} else if matchCode {
			if newCode {
				newSeg := Segment{Docs: "", Code: line}
				segs = append(segs, &newSeg)
			} else {
				segs[len(segs)-1].Code = segs[len(segs)-1].Code + "\n" + line
			}
			debug("CODE: " + line)
			lastSeen = "code"
		}
	}

	for i, seg := range segs {
		seg.CodeEmpty = (seg.Code == "")
		seg.CodeLeading = (i < (len(segs) - 1))
		seg.CodeRun = strings.Contains(seg.Code, "package main")
	}

	return segs, filecontent
}

func chromaFormat(code, filePath string) string {
	lexer := lexers.Get(filePath)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	if strings.HasSuffix(filePath, ".sh") {
		lexer = SimpleShellOutputLexer
	}

	lexer = chroma.Coalesce(lexer)

	style := styles.Get("swapoff")
	if style == nil {
		style = styles.Fallback
	}

	formatter := html.New(html.WithClasses(true))
	iterator, err := lexer.Tokenise(nil, string(code))
	check(err)
	buf := new(bytes.Buffer)
	err = formatter.Format(buf, style, iterator)
	check(err)
	return buf.String()
}

func parseAndRenderSegments(sourcePath string) ([]*Segment, string) {
	segs, filecontent := parseSegments(sourcePath)
	lexer := whichLexer(sourcePath)

	for _, seg := range segs {
		if seg.Docs != "" {
			seg.DocsRendered = markdown(seg.Docs)
		}
		if seg.Code != "" {
			seg.CodeRendered = chromaFormat(seg.Code, sourcePath)

			// adding the content to the js code for copying to the clipboard
			if strings.HasSuffix(sourcePath, ".go") {
				seg.CodeForJs = strings.Trim(seg.Code, "\n") + "\n"
			}
		}
	}

	// we are only interested in the 'go' code to pass to play.golang.org
	if lexer != "go" {
		filecontent = ""
	}

	return segs, filecontent
}

func openCategoryFile(dir string) *Category {
	path := dir + "/meta.yaml"
	b, err := ioutil.ReadFile(path)
	check(err)
	var c Category
	err = yaml.Unmarshal(b, &c)
	check(err)
	c.ID = filepath.Base(dir)
	return &c
}

func parseExampleMeta(path string) (*ExampleMeta, error) {
}

func parseExamples() []*Categories {
	categories := make(map[string]*Category)

	err := fs.WalkDir("examples/", fs.WalkDirFunc(func(path string, dir fs.DirEntry, err error) error {
		// Ignore files.
		if !dir.IsDir() {
			return nil
		}

		if isExampleDir(path) {

			metaPath := filepath.Join(path, "meta.yaml")
			ioutil.ReadFile(metaPath)
		}

	}))
	check(err)


		exDir := filepath.Dir(examplePath)
		categoryID := filepath.Base(exDir)
		c, ok := categories[categoryID]
		if !ok {
			c = openCategoryFile(exDir)
			categories[categoryID] = c
		}

		example := Example{
			Name:     exampleName,
			Category: category.ID,
		}

		example.ID = examplePath
		example.Segments = make([][]*Segment, 0)
		sourcePaths := mustGlob("examples/" + examplePath + "/*")

		for _, sourcePath := range sourcePaths {
			if !isDir(sourcePath) {
				if !strings.HasSuffix(sourcePath, ".md") {
					sourceSegs, filecontents := parseAndRenderSegments(sourcePath)
					if filecontents != "" {
						example.GoCode = filecontents
					}
					example.Segments = append(example.Segments, sourceSegs)
				}
			}
		}

		newCodeHash := sha1Sum(example.GoCode)
		if example.GoCodeHash != newCodeHash {
			example.URLHash = resetURLHashFile(newCodeHash, example.GoCode, "examples/"+example.ID+"/"+example.ID+".hash")
		}
		examples = append(examples, &example)
	}

	for i, example := range examples {
		if i > 0 {
			example.PrevExample = examples[i-1]
		}
		if i < (len(examples) - 1) {
			example.NextExample = examples[i+1]
		}
	}

	return categories, examples
}

func renderIndex(categories []*Category, examples []*Example) {
	if verbose() {
		fmt.Println("Rendering index")
	}
	indexTmpl := template.New("index")
	template.Must(indexTmpl.Parse(mustReadFile("templates/footer.tmpl")))
	template.Must(indexTmpl.Parse(mustReadFile("templates/index.tmpl")))
	indexF, err := os.Create(siteDir + "/index.html")
	check(err)
	defer indexF.Close()
	check(indexTmpl.Execute(indexF, examples))
}

func renderExamples(categories []*Category, examples []*Example) {
	if verbose() {
		fmt.Println("Rendering examples")
	}

	exampleTmpl := template.New("example")
	template.Must(exampleTmpl.Parse(mustReadFile("templates/footer.tmpl")))
	template.Must(exampleTmpl.Parse(mustReadFile("templates/example.tmpl")))

	for _, example := range examples {
		name := filepath.Base(example.ID)
		dir := filepath.Join(siteDir, filepath.Dir(example.ID))
		err := os.MkdirAll(dir, 0755)
		check(err)
		exampleF, err := os.Create(dir + "/" + name)
		check(err)
		defer exampleF.Close()
		check(exampleTmpl.Execute(exampleF, example))
	}
}

func render404() {
	if verbose() {
		fmt.Println("Rendering 404")
	}
	tmpl := template.New("404")
	template.Must(tmpl.Parse(mustReadFile("templates/footer.tmpl")))
	template.Must(tmpl.Parse(mustReadFile("templates/404.tmpl")))
	file, err := os.Create(siteDir + "/404.html")
	check(err)
	defer file.Close()
	check(tmpl.Execute(file, ""))
}

func main() {
	if len(os.Args) > 1 {
		siteDir = os.Args[1]
	}

	ensureDir(siteDir)

	copyFile("templates/site.css", siteDir+"/site.css")
	copyFile("templates/site.js", siteDir+"/site.js")
	copyFile("templates/favicon.ico", siteDir+"/favicon.ico")
	copyFile("templates/play.png", siteDir+"/play.png")
	copyFile("templates/clipboard.png", siteDir+"/clipboard.png")

	categories, examples := parseExamples()
	renderIndex(categories, examples)
	renderExamples(categories, examples)
	render404()
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
