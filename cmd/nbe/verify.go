package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Verification of examples.
//
// `nbe verify` builds and runs each example client the same way `nbe run`
// does and then checks the result:
//
//   - fail: the image did not build, the run exited non-zero, or timed out.
//   - warn: the run succeeded, but the output looks suspicious compared to the
//     committed output.txt: an error marker (panic, traceback, ...) appeared
//     that was not in the committed output, the output is empty, or the
//     normalized output drifted significantly from the committed output.
//   - pass: everything else.
//
// The committed output.txt is the record of what the example is documented
// to do, so drift is the signal that the behavior of the demo changed and a
// human (or an agent) needs to check that the prose in meta.yaml and the code
// comments still match.

const (
	VerifyPass = "pass"
	VerifyWarn = "warn"
	VerifyFail = "fail"
)

type VerifyResult struct {
	Example      string   `json:"example"`
	Lang         string   `json:"lang"`
	Status       string   `json:"status"`
	Stage        string   `json:"stage,omitempty"`
	Error        string   `json:"error,omitempty"`
	Reasons      []string `json:"reasons,omitempty"`
	BuildSeconds float64  `json:"build_seconds"`
	RunSeconds   float64  `json:"run_seconds"`
	// Similarity of the normalized output to the committed output.txt in
	// the range [0, 1]. -1 if there is no committed output to compare to.
	Similarity float64  `json:"similarity"`
	NewMarkers []string `json:"new_markers,omitempty"`
	// Tail of the build log or run output, for failures and warnings.
	Tail string `json:"tail,omitempty"`
}

type Verifier struct {
	Repo        string
	Concurrency int
	Timeout     time.Duration
	// Minimum similarity to the committed output before warning.
	MinSimilarity float64
	// If set, the new output (and build log on failure) is written here.
	OutDir string
	// If true, re-record output.cast/output.txt for examples that did not
	// fail and whose output changed (or all of them, with RecordAll).
	Record    bool
	RecordAll bool
	// Progress is written here.
	Log func(format string, args ...any)
}

// resolveVerifyTargets turns arguments into a sorted list of example client
// directories (relative to the repo). An argument can be "all", a language
// directory name such as "go", or a glob/path such as
// "examples/messaging/*/rust" ("examples/" may be omitted).
func resolveVerifyTargets(repo string, args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{"all"}
	}
	seen := map[string]struct{}{}
	for _, a := range args {
		var pattern string
		switch {
		case a == "all":
			pattern = "examples/*/*/*"
		case strings.ContainsAny(a, "/*?["):
			pattern = strings.TrimSuffix(a, "/")
			if !strings.HasPrefix(pattern, "examples/") {
				pattern = filepath.Join("examples", pattern)
			}
		default:
			pattern = filepath.Join("examples", "*", "*", a)
		}
		ms, err := filepath.Glob(filepath.Join(repo, pattern))
		if err != nil {
			return nil, err
		}
		if len(ms) == 0 {
			return nil, fmt.Errorf("%q matches no examples", a)
		}
		for _, m := range ms {
			if fi, err := os.Stat(m); err != nil || !fi.IsDir() {
				continue
			}
			if !isRunnableClient(m) {
				continue
			}
			rel, err := filepath.Rel(repo, m)
			if err != nil {
				return nil, err
			}
			// Only client directories, i.e. examples/<category>/<example>/<client>.
			if len(strings.Split(filepath.ToSlash(rel), "/")) != 4 {
				continue
			}
			seen[filepath.ToSlash(rel)] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// isRunnableClient reports whether a client directory has something nbe can
// run: a main.* (or Main.*) entrypoint or its own Dockerfile. Some directories (e.g. a
// shell example made of a README and scripts) are documentation only.
func isRunnableClient(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
		return true
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		// main.go, main.sh, Main.java, Main.cs, ...
		if !e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "main.") {
			return true
		}
	}
	return false
}

func (v *Verifier) Run(examples []string) []*VerifyResult {
	if v.Concurrency < 1 {
		v.Concurrency = 1
	}
	if v.Log == nil {
		v.Log = func(string, ...any) {}
	}

	results := make([]*VerifyResult, len(examples))
	work := make(chan int)
	wg := sync.WaitGroup{}
	for i := 0; i < v.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range work {
				r := v.verifyOne(examples[idx])
				results[idx] = r
				v.Log("%-4s %s (build %.0fs, run %.0fs%s)%s",
					strings.ToUpper(r.Status), r.Example, r.BuildSeconds, r.RunSeconds,
					similarityLabel(r.Similarity), reasonsLabel(r))
			}
		}()
	}
	for i := range examples {
		work <- i
	}
	close(work)
	wg.Wait()
	return results
}

func similarityLabel(s float64) string {
	if s < 0 {
		return ""
	}
	return fmt.Sprintf(", similarity %.2f", s)
}

func reasonsLabel(r *VerifyResult) string {
	if r.Error == "" && len(r.Reasons) == 0 {
		return ""
	}
	parts := append([]string{}, r.Reasons...)
	if r.Error != "" {
		parts = append([]string{r.Error}, parts...)
	}
	return ": " + strings.Join(parts, "; ")
}

func (v *Verifier) verifyOne(example string) *VerifyResult {
	r := &VerifyResult{
		Example:    example,
		Lang:       filepath.Base(example),
		Similarity: -1,
	}

	var (
		buildLog *bytes.Buffer
		image    string
		err      error
	)
	start := time.Now()
	// Retry builds that failed for transient registry/network reasons so
	// that rate limits are not reported as broken examples.
	for attempt := 1; ; attempt++ {
		buildLog = bytes.NewBuffer(nil)
		b := ImageBuilder{
			Repo:    v.Repo,
			Example: example,
			Verbose: true,
			Stdout:  buildLog,
			Stderr:  buildLog,
		}
		image, err = b.Run()
		if err == nil || attempt == 4 || !transientBuildRe.MatchString(buildLog.String()) {
			break
		}
		wait := time.Duration(attempt*attempt) * 20 * time.Second
		v.Log("%s: transient build failure, retrying in %s", example, wait)
		time.Sleep(wait)
	}
	r.BuildSeconds = time.Since(start).Seconds()
	if err != nil {
		r.Status = VerifyFail
		r.Stage = "build"
		r.Error = err.Error()
		r.Tail = tailLines(buildLog.String(), 60)
		v.writeOut(example, "build.log", buildLog.Bytes())
		return r
	}
	defer removeImage(image)

	var runOut *bytes.Buffer
	start = time.Now()
	// Retry runs that failed because a service image (e.g. nats) could not
	// be pulled.
	for attempt := 1; ; attempt++ {
		runOut = bytes.NewBuffer(nil)
		cr := ComposeRunner{
			Repo:    v.Repo,
			Example: example,
			NoAnsi:  true,
			Stdout:  runOut,
			Stderr:  runOut,
			Timeout: v.Timeout,
		}
		err = cr.Run(image)
		if err == nil || attempt == 3 || !transientBuildRe.MatchString(runOut.String()) {
			break
		}
		wait := time.Duration(attempt*attempt) * 20 * time.Second
		v.Log("%s: transient run failure, retrying in %s", example, wait)
		time.Sleep(wait)
	}
	r.RunSeconds = time.Since(start).Seconds()

	output := string(removeComposeLines(runOut.Bytes()))
	v.writeOut(example, "output.txt", []byte(output))

	if err != nil {
		r.Status = VerifyFail
		r.Stage = "run"
		r.Error = err.Error()
		r.Tail = tailLines(output, 60)
		return r
	}

	r.Status = VerifyPass

	committed, cerr := os.ReadFile(filepath.Join(v.Repo, example, "output.txt"))
	if strings.TrimSpace(output) == "" {
		r.Status = VerifyWarn
		r.Reasons = append(r.Reasons, "empty output")
	}

	newMarkers := errorMarkers(output)
	if cerr == nil {
		old := map[string]struct{}{}
		for _, m := range errorMarkers(string(committed)) {
			old[m] = struct{}{}
		}
		var fresh []string
		for _, m := range newMarkers {
			if _, ok := old[m]; !ok {
				fresh = append(fresh, m)
			}
		}
		newMarkers = fresh

		r.Similarity = outputSimilarity(string(committed), output)
		if r.Similarity < v.MinSimilarity {
			r.Status = VerifyWarn
			r.Reasons = append(r.Reasons, fmt.Sprintf("output drifted from committed output.txt (similarity %.2f)", r.Similarity))
		}
	}
	if len(newMarkers) > 0 {
		r.Status = VerifyWarn
		r.NewMarkers = newMarkers
		r.Reasons = append(r.Reasons, "new error markers: "+strings.Join(newMarkers, ", "))
	}
	if r.Status != VerifyPass {
		r.Tail = tailLines(output, 60)
	}

	// Skip re-recording when nothing but volatile values (timestamps, IDs)
	// changed, to keep diffs reviewable.
	if v.Record && (v.RecordAll || r.Similarity < 1) {
		if err := recordWithImage(v.Repo, example, image); err != nil {
			r.Reasons = append(r.Reasons, fmt.Sprintf("recording failed: %s", err))
			if r.Status == VerifyPass {
				r.Status = VerifyWarn
			}
		}
	}

	return r
}

// Build errors caused by the registry or network rather than the example.
var transientBuildRe = regexp.MustCompile(`429 Too Many Requests|status code 429|failed to export image: No such image|toomanyrequests|TLS handshake timeout|i/o timeout|connection reset by peer|unexpected EOF|503 Service Unavailable|502 Bad Gateway`)

func (v *Verifier) writeOut(example, name string, data []byte) {
	if v.OutDir == "" {
		return
	}
	dir := filepath.Join(v.OutDir, example)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	os.WriteFile(filepath.Join(dir, name), data, 0644)
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// Markers that usually indicate an example broke at runtime, keyed by a
// short name used in reports.
var errorMarkerRes = []struct {
	name string
	re   *regexp.Regexp
}{
	{"go panic", regexp.MustCompile(`(?m)^panic: `)},
	{"rust panic", regexp.MustCompile(`panicked at`)},
	{"python traceback", regexp.MustCompile(`Traceback \(most recent call last\)`)},
	{"java exception", regexp.MustCompile(`Exception in thread|\tat [a-zA-Z_$][\w$.]*\(`)},
	{".net unhandled exception", regexp.MustCompile(`Unhandled exception`)},
	{"js uncaught error", regexp.MustCompile(`Uncaught|UnhandledPromiseRejection|node:internal`)},
	{"elixir error", regexp.MustCompile(`(?m)^\*\* \(\w`)},
	{"crystal error", regexp.MustCompile(`(?m)^Unhandled exception`)},
	{"segfault", regexp.MustCompile(`(?i)segmentation fault`)},
	{"nats timeout", regexp.MustCompile(`nats: timeout|TimeoutError|context deadline exceeded`)},
	{"nats cli error", regexp.MustCompile(`(?m)^nats: error:`)},
	{"command not found", regexp.MustCompile(`command not found|: not found`)},
	{"unknown flag", regexp.MustCompile(`unknown (long )?flag|unknown command|unexpected argument`)},
}

func errorMarkers(s string) []string {
	var out []string
	for _, m := range errorMarkerRes {
		if m.re.MatchString(s) {
			out = append(out, m.name)
		}
	}
	return out
}

var (
	ansiRe      = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\][^\x07]*\x07`)
	timestampRe = regexp.MustCompile(`\d{4}[-/]\d{2}[-/]\d{2}([T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?)?|\d{2}:\d{2}:\d{2}(\.\d+)?`)
	uuidRe      = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	// NUIDs, nkeys, JWT segments, hashes, ...
	idRe = regexp.MustCompile(`[A-Za-z0-9_\-+/=]{16,}`)
	// Short random identifiers such as the "7Q3G" in "EVENTS_7Q3G".
	shortIDRe = regexp.MustCompile(`[A-Za-z0-9]{4,}`)
	// Random upper-case suffixes such as the "PCLB" in "PLAIN_PCLB".
	suffixIDRe = regexp.MustCompile(`_[A-Z0-9]{4}\b`)
	numberRe   = regexp.MustCompile(`\d+(\.\d+)?`)
	spaceRe    = regexp.MustCompile(`\s+`)
)

func normalizeOutput(s string) []string {
	s = ansiRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r", "")
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = timestampRe.ReplaceAllString(line, "<T>")
		line = uuidRe.ReplaceAllString(line, "<ID>")
		line = idRe.ReplaceAllStringFunc(line, func(m string) string {
			// Keep long plain words (e.g. subjects/identifiers) that have no digits.
			if !strings.ContainsAny(m, "0123456789") {
				return m
			}
			return "<ID>"
		})
		line = suffixIDRe.ReplaceAllString(line, "_<ID>")
		line = shortIDRe.ReplaceAllStringFunc(line, func(m string) string {
			if strings.ContainsAny(m, "0123456789") && strings.IndexFunc(m, isLetter) >= 0 {
				return "<ID>"
			}
			return m
		})
		line = numberRe.ReplaceAllString(line, "<N>")
		line = strings.TrimSpace(spaceRe.ReplaceAllString(line, " "))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// outputSimilarity returns the Dice coefficient of the longest common
// subsequence of normalized lines: 1 means identical, 0 means nothing shared.
func outputSimilarity(a, b string) float64 {
	la := normalizeOutput(a)
	lb := normalizeOutput(b)
	if len(la) == 0 && len(lb) == 0 {
		return 1
	}
	if len(la) == 0 || len(lb) == 0 {
		return 0
	}
	// Bound the work for very large outputs.
	const max = 3000
	if len(la) > max {
		la = la[:max]
	}
	if len(lb) > max {
		lb = lb[:max]
	}
	prev := make([]int, len(lb)+1)
	cur := make([]int, len(lb)+1)
	for i := 1; i <= len(la); i++ {
		for j := 1; j <= len(lb); j++ {
			if la[i-1] == lb[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	return 2 * float64(prev[len(lb)]) / float64(len(la)+len(lb))
}

func writeVerifyJSON(path string, results []*VerifyResult) error {
	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func readVerifyJSON(paths []string) ([]*VerifyResult, error) {
	var all []*VerifyResult
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var rs []*VerifyResult
		if err := json.Unmarshal(b, &rs); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		all = append(all, rs...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Example < all[j].Example })
	return all, nil
}

// verifyMarkdown renders a report suitable for a GitHub issue, PR comment or
// job summary.
func verifyMarkdown(results []*VerifyResult, title string) string {
	buf := bytes.NewBuffer(nil)
	if title != "" {
		fmt.Fprintf(buf, "## %s\n\n", title)
	}

	type counts struct{ pass, warn, fail int }
	byLang := map[string]*counts{}
	var langs []string
	total := counts{}
	for _, r := range results {
		c, ok := byLang[r.Lang]
		if !ok {
			c = &counts{}
			byLang[r.Lang] = c
			langs = append(langs, r.Lang)
		}
		switch r.Status {
		case VerifyPass:
			c.pass++
			total.pass++
		case VerifyWarn:
			c.warn++
			total.warn++
		default:
			c.fail++
			total.fail++
		}
	}
	sort.Strings(langs)

	fmt.Fprintf(buf, "**%d examples:** ✅ %d pass · ⚠️ %d warn · ❌ %d fail\n\n", len(results), total.pass, total.warn, total.fail)
	fmt.Fprintf(buf, "| Language | ✅ Pass | ⚠️ Warn | ❌ Fail |\n|---|---:|---:|---:|\n")
	for _, l := range langs {
		c := byLang[l]
		fmt.Fprintf(buf, "| %s | %d | %d | %d |\n", l, c.pass, c.warn, c.fail)
	}
	buf.WriteString("\n")

	section := func(status, heading string) {
		var rs []*VerifyResult
		for _, r := range results {
			if r.Status == status {
				rs = append(rs, r)
			}
		}
		if len(rs) == 0 {
			return
		}
		fmt.Fprintf(buf, "### %s\n\n", heading)
		for _, r := range rs {
			summary := r.Example
			if r.Stage != "" {
				summary += " — " + r.Stage
			}
			fmt.Fprintf(buf, "<details><summary><code>%s</code></summary>\n\n", summary)
			if r.Error != "" {
				fmt.Fprintf(buf, "- error: `%s`\n", strings.ReplaceAll(r.Error, "`", "'"))
			}
			for _, reason := range r.Reasons {
				fmt.Fprintf(buf, "- %s\n", reason)
			}
			if r.Tail != "" {
				tail := r.Tail
				// Keep issue bodies within GitHub's size limit.
				if len(tail) > 4000 {
					tail = "…" + tail[len(tail)-4000:]
				}
				fmt.Fprintf(buf, "\n```\n%s\n```\n", strings.ReplaceAll(tail, "```", "'''"))
			}
			buf.WriteString("\n</details>\n\n")
		}
	}
	section(VerifyFail, "❌ Failures")
	section(VerifyWarn, "⚠️ Warnings (ran, but output needs a look)")

	return buf.String()
}
