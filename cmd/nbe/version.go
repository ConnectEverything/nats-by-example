package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
	"gopkg.in/yaml.v3"
)

const (
	dockerDir   = "docker"
	examplesDir = "examples"
)

type MultiErr []error

func (m *MultiErr) Empty() bool {
	for _, err := range *m {
		if err != nil {
			return false
		}
	}
	return true
}

func (m *MultiErr) Error() string {
	var toks []string
	for _, err := range *m {
		if err != nil {
			toks = append(toks, err.Error())
		}
	}
	if len(toks) > 0 {
		return strings.Join(toks, "\n")
	}
	return ""
}

type Versions struct {
	Server    string
	CLI       string
	Go        string
	Python    string
	Deno      string
	Node      string
	WebSocket string
	Rust      string
	Java      string
	DotNet    string
}

type Matrix struct {
	Server    []string
	CLI       []string
	Go        []string
	Python    []string
	Deno      []string
	Node      []string
	WebSocket []string
	Rust      []string
	Java      []string
	DotNet    []string
}

func openVersionsFile(path string) (*Versions, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var v Versions
	err = yaml.Unmarshal(b, &v)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &v, nil
}

func openMatrixFile(path string) (*Matrix, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var m Matrix
	err = yaml.Unmarshal(b, &m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

func findAndReplace(re *regexp.Regexp, rep string, path string) error {
	st, err := os.Stat(path)
	if err != nil {
		// Ignore if the file does not exist.
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	perm := st.Mode().Perm()
	src, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	if matches := re.FindAll(src, -1); len(matches) == 0 {
		return nil
		//return fmt.Errorf("%s: no matches found", path)
	}

	out := re.ReplaceAll(src, []byte(rep))
	return ioutil.WriteFile(path, out, perm)
}

func findAndReplaceMulti(re *regexp.Regexp, rep string, paths ...string) error {
	for _, p := range paths {
		if err := findAndReplace(re, rep, p); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}

var (
	composeNatsImageRe = regexp.MustCompile(`image: (docker.io/)?nats:\d+\.\d+\.\d+`)
	composeNatsImageT  = `image: docker.io/nats:%s`

	dockerNatsGoRe = regexp.MustCompile(`nats-server/v2@v\d+\.\d+\.\d+`)
	dockerNatsGoT  = `nats-server/v2@v%s`

	dockerCliGoRe = regexp.MustCompile(`natscli/nats@v\d+\.\d+\.\d+`)
	dockerCliGoT  = `natscli/nats@v%s`

	goDepRe = regexp.MustCompile(`nats.go v\d+\.\d+\.\d+`)
	goDepT  = `nats.go v%s`

	pythonDepRe = regexp.MustCompile(`nats-py\[nkeys\]==\d+\.\d+\.\d+`)
	pythonDepT  = `nats-py[nkeys]==%s`

	nodeDepRe = regexp.MustCompile(`"nats": "\^\d+\.\d+\.\d+"`)
	nodeDepT  = `"nats": "^%s"`

	denoDepRe = regexp.MustCompile(`x/nats@v\d+\.\d+\.\d+`)
	denoDepT  = `x/nats@v%s`

	rustDepRe = regexp.MustCompile(`async-nats = "\d+\.\d+\.\d+"`)
	rustDepT  = `async-nats = "%s"`

	javaDepRe = regexp.MustCompile(`io.nats:jnats:\d+\.\d+\.\d+`)
	javaDepT  = `io.nats:jnats:%s`

	dotnetDepRe = regexp.MustCompile(`Version="\d+\.\d+\.\d+"`)
	dotnetDepT  = `Version="%s"`
)

func setComposeServerVersion(version string, base string) error {
	err := MultiErr{
		findAndReplaceMulti(
			composeNatsImageRe,
			fmt.Sprintf(composeNatsImageT, version),
			filepath.Join(base, "docker-compose.yaml"),
			filepath.Join(base, "docker-compose.cluster.yaml"),
		),
	}
	if err.Empty() {
		return nil
	}
	return &err
}

func setCLIVersion(cliVersion string, serverVersion string, base string) error {
	err := MultiErr{
		findAndReplaceMulti(
			dockerCliGoRe,
			fmt.Sprintf(dockerCliGoT, cliVersion),
			filepath.Join(base, "Dockerfile"),
		),
		findAndReplaceMulti(
			dockerNatsGoRe,
			fmt.Sprintf(dockerNatsGoT, serverVersion),
			filepath.Join(base, "Dockerfile"),
		),
	}
	if err.Empty() {
		return nil
	}
	return &err
}

func setGoVersion(version string, base string) error {
	if _, err := os.Stat(filepath.Join(base, "go.mod")); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	err := MultiErr{
		findAndReplaceMulti(
			goDepRe,
			fmt.Sprintf(goDepT, version),
			filepath.Join(base, "go.mod"),
		),
	}
	if !err.Empty() {
		return &err
	}
	cmd := exec.Command("go", "get")
	cmd.Dir = base
	return cmd.Run()
}

func setPythonVersion(version string, base string) error {
	err := MultiErr{
		findAndReplaceMulti(
			pythonDepRe,
			fmt.Sprintf(pythonDepT, version),
			filepath.Join(base, "requirements.txt"),
		),
	}
	if err.Empty() {
		return nil
	}
	return &err
}

func setNodeVersion(version string, base string) error {
	err := MultiErr{
		findAndReplaceMulti(
			nodeDepRe,
			fmt.Sprintf(nodeDepT, version),
			filepath.Join(base, "package.json"),
		),
	}
	if err.Empty() {
		return nil
	}
	return &err
}

func setDenoVersion(version string, base string) error {
	err := MultiErr{
		findAndReplaceMulti(
			denoDepRe,
			fmt.Sprintf(denoDepT, version),
			filepath.Join(base, "main.deno"),
		),
	}
	if err.Empty() {
		return nil
	}
	return &err
}

func setDenoAllVersion(version string, base string) error {
	matches, err := filepath.Glob(fmt.Sprintf("%s/*/*/deno/main.ts", base))
	if err != nil {
		return fmt.Errorf("%s: %w", base, err)
	}

	var errs MultiErr
	for _, m := range matches {
		errs = append(errs, findAndReplaceMulti(
			denoDepRe,
			fmt.Sprintf(denoDepT, version),
			m,
		))
	}
	if errs.Empty() {
		return nil
	}
	return &errs
}

func setRustVersion(version string, base string) error {
	err := MultiErr{
		findAndReplaceMulti(
			rustDepRe,
			fmt.Sprintf(rustDepT, version),
			filepath.Join(base, "Cargo.toml"),
		),
	}
	if err.Empty() {
		return nil
	}
	return &err
}

func setJavaVersion(version string, base string) error {
	err := MultiErr{
		findAndReplaceMulti(
			javaDepRe,
			fmt.Sprintf(javaDepT, version),
			filepath.Join(base, "build.gradle"),
		),
	}
	if err.Empty() {
		return nil
	}
	return &err
}

func setDotNetVersion(version string, base string) error {
	err := MultiErr{
		findAndReplaceMulti(
			dotnetDepRe,
			fmt.Sprintf(dotnetDepT, version),
			filepath.Join(base, "example.csproj"),
		),
	}
	if err.Empty() {
		return nil
	}
	return &err
}

func setVersions(path string) error {
	vs, err := openVersionsFile(path)
	if err != nil {
		return err
	}
	errs := MultiErr{
		setComposeServerVersion(vs.Server, ""),
		setCLIVersion(vs.CLI, vs.Server, filepath.Join(dockerDir, "cli")),
		setGoVersion(vs.Go, filepath.Join(dockerDir, "go")),
		setPythonVersion(vs.Python, filepath.Join(dockerDir, "python")),
		setNodeVersion(vs.Node, filepath.Join(dockerDir, "node")),
		setDenoAllVersion(vs.Deno, examplesDir),
		setRustVersion(vs.Rust, filepath.Join(dockerDir, "rust")),
		setJavaVersion(vs.Java, filepath.Join(dockerDir, "java")),
		setDotNetVersion(vs.DotNet, filepath.Join(dockerDir, "dotnet")),
	}
	if errs.Empty() {
		return nil
	}
	return &errs
}

func replaceVersions(dir string, vs *Versions) error {
	errs := MultiErr{
		setComposeServerVersion(vs.Server, dir),
		setCLIVersion(vs.CLI, vs.Server, dir),
		setGoVersion(vs.Go, dir),
		setPythonVersion(vs.Python, dir),
		setNodeVersion(vs.Node, dir),
		setDenoVersion(vs.Deno, dir),
		setRustVersion(vs.Rust, dir),
		setJavaVersion(vs.Java, dir),
		setDotNetVersion(vs.DotNet, dir),
	}
	if errs.Empty() {
		return nil
	}
	return &errs
}

type Job struct {
	Repo          string
	Example       string
	Client        string
	ClientVersion string
	ServerVersion string
	Verbose       bool
}

func (j *Job) Run() error {
	vs := &Versions{
		Server: j.ServerVersion,
	}

	switch j.Client {
	case "cli":
		vs.CLI = j.ClientVersion
	case "go":
		vs.Go = j.ClientVersion
	case "python":
		vs.Python = j.ClientVersion
	case "deno":
		vs.Deno = j.ClientVersion
	case "node":
		vs.Node = j.ClientVersion
	case "websocket":
		vs.WebSocket = j.ClientVersion
	case "rust":
		vs.Rust = j.ClientVersion
	case "java":
		vs.Java = j.ClientVersion
	case "dotnet":
		vs.DotNet = j.ClientVersion
	}

	b := ImageBuilder{
		Repo:     j.Repo,
		Example:  j.Example,
		Versions: vs,
		Verbose:  j.Verbose,
	}

	image, err := b.Run()
	if err != nil {
		return err
	}
	defer removeImage(image)

	stderr := bytes.NewBuffer(nil)
	r := ComposeRunner{
		Repo:     j.Repo,
		Example:  j.Example,
		Versions: vs,
		Verbose:  j.Verbose,
		Stderr:   stderr,
		Stdout:   stderr,
	}
	err = r.Run(image)
	if err != nil {
		return fmt.Errorf("%w:\n%s", err, stderr.String())
	}
	return nil
}

type Result struct {
	Example       string
	ServerVersion string
	Client        string
	ClientVersion string
	Error         error
}

func runMatrix(workers int, path string, repo string, examples []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m, err := openMatrixFile(path)
	if err != nil {
		return err
	}

	wg := &sync.WaitGroup{}
	wg.Add(workers)
	workch := make(chan *Job, workers)
	resch := make(chan *Result, 1000)

	// Start the workers.
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case j, ok := <-workch:
					if !ok {
						return
					}

					err := j.Run()

					fmt.Fprintf(os.Stderr, "* %q: server (%s) x %s (%s)\n", j.Example, j.ServerVersion, j.Client, j.ClientVersion)
					r := &Result{
						Example:       j.Example,
						ServerVersion: j.ServerVersion,
						Client:        j.Client,
						ClientVersion: j.ClientVersion,
						Error:         err,
					}

					select {
					case resch <- r:
					case <-time.After(time.Second):
						log.Printf("timeout sending on result channel")
						return
					}
				}
			}
		}(i)
	}

	// Prepare the jobs to queue up.
	cm := make(map[string][]string)

	t0 := time.Now()

	for _, e := range examples {
		client := filepath.Base(e)

		var versions []string
		switch client {
		case "cli":
			versions = m.CLI
		case "go":
			versions = m.Go
		case "python":
			versions = m.Python
		case "deno":
			versions = m.Deno
		case "node":
			versions = m.Node
		case "websocket":
			versions = m.WebSocket
		case "rust":
			versions = m.Rust
		case "java":
			versions = m.Java
		case "dotnet":
			versions = m.DotNet
		default:
			return fmt.Errorf("unknown client: %s", client)
		}

		cm[client] = versions

		for _, s := range m.Server {
			for _, c := range versions {
				workch <- &Job{
					Repo:          repo,
					Example:       e,
					Client:        client,
					ServerVersion: s,
					ClientVersion: c,
				}
			}
		}
	}

	close(workch)
	go func() {
		wg.Wait()
		fmt.Fprintf(os.Stderr, "Total time... %s\n", time.Since(t0))
		close(resch)
	}()

	results := make(map[string]map[string]map[[2]string]*Result)

	for r := range resch {
		e, ok := results[r.Client]
		if !ok {
			e = make(map[string]map[[2]string]*Result)
			results[r.Client] = e
		}
		m, ok := e[r.Example]
		if !ok {
			m = make(map[[2]string]*Result)
			e[r.Example] = m
		}
		k := [2]string{r.ServerVersion, r.ClientVersion}
		m[k] = r
	}

	for _, c := range languageOrder {
		rs, ok := results[c]
		if !ok {
			continue
		}

		clientVersions := cm[c]

		fmt.Printf("# %s\n", availableLanguages[c])

		for e, cr := range rs {
			fmt.Printf("## %s\n", e)

			tw := tablewriter.NewWriter(os.Stdout)
			head := append([]string{""}, clientVersions...)
			tw.SetHeader(head)

			// Each version is a row.
			for _, s := range m.Server {
				row := []string{s}

				for _, c := range clientVersions {
					k := [2]string{s, c}
					r := cr[k]
					if r.Error == nil {
						row = append(row, "OK")
					} else {
						log.Println(r.Error)
						row = append(row, "ERR")
					}
				}

				tw.Append(row)
			}

			tw.Render()
		}
	}

	return nil
}
