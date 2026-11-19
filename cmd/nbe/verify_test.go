package main

import (
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOutputSimilarityIgnoresVolatileValues(t *testing.T) {
	a := `2024-07-11T17:24:14Z received message seq=1 id=NUID4abcDEF0123456789xyz
took 12.5ms
inbox _INBOX.Zr8cKjS5JuPjVqzNMEO1Ab
`
	b := "\x1b[32m2026-11-19T10:00:00Z received message seq=7 id=NUIDzzzzzzzzzzz98765432a\x1b[0m\r\ntook 3.1ms\n" +
		"inbox _INBOX.H8oPq2Lm0VbXcRt5Yu7Wq9\n"
	if s := outputSimilarity(a, b); s != 1 {
		t.Fatalf("expected identical after normalization, got %.2f\n%q\n%q", s, normalizeOutput(a), normalizeOutput(b))
	}
}

func TestOutputSimilarityIgnoresShortRandomIDs(t *testing.T) {
	a := "Stream: EVENTS_7Q3G has these subjects: 'PLAIN_7PKV,GT_7PR2.>'\n"
	b := "Stream: EVENTS_H6LN has these subjects: 'PLAIN_PCLB,GT_H6AT.>'\n"
	if s := outputSimilarity(a, b); s != 1 {
		t.Fatalf("got %.2f: %q vs %q", s, normalizeOutput(a), normalizeOutput(b))
	}
}

func TestOutputSimilarityDetectsDrift(t *testing.T) {
	a := "published 3 messages\nreceived greet.joe\nreceived greet.pam\nreceived greet.sue\n"
	b := "connecting\nerror: unknown method\n"
	if s := outputSimilarity(a, b); s > 0.2 {
		t.Fatalf("expected low similarity, got %.2f", s)
	}
	if s := outputSimilarity(a, ""); s != 0 {
		t.Fatalf("expected 0 for empty output, got %.2f", s)
	}
}

func TestErrorMarkers(t *testing.T) {
	cases := map[string][]string{
		"panic: runtime error\n":                             {"go panic"},
		"Traceback (most recent call last):\n  File x":       {"python traceback"},
		"thread 'main' panicked at src/main.rs:3:5":          {"rust panic"},
		"nats: error: unknown flag --foo":                    {"nats cli error", "unknown flag"},
		"all good\nreceived 3 messages\n":                    nil,
		"Exception in thread \"main\" java.lang.Error: boom": {"java exception"},
	}
	for in, want := range cases {
		if got := errorMarkers(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", in, got, want)
		}
	}
}

func TestResolveVerifyTargets(t *testing.T) {
	repo := t.TempDir()
	for _, d := range []string{
		"examples/messaging/pub-sub/go",
		"examples/messaging/pub-sub/rust",
		"examples/kv/intro/go",
		"examples/auth/callout/cli/service",
		"examples/ops/docs-only/shell",
		"examples/messaging/pub-sub/java",
	} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0755); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(d, "docs-only/shell") {
			name := "main.txt"
			if strings.HasSuffix(d, "/java") {
				name = "Main.java"
			}
			if err := os.WriteFile(filepath.Join(repo, d, name), nil, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	got, err := resolveVerifyTargets(repo, []string{"go"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"examples/kv/intro/go", "examples/messaging/pub-sub/go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	got, err = resolveVerifyTargets(repo, []string{"messaging/*/*", "go"})
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"examples/kv/intro/go", "examples/messaging/pub-sub/go", "examples/messaging/pub-sub/java", "examples/messaging/pub-sub/rust"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	got, _ = resolveVerifyTargets(repo, nil)
	// cli/service is not a client dir, docs-only/shell has nothing to run,
	// and examples/auth/callout/cli has no main file of its own here.
	if len(got) != 4 {
		t.Fatalf("all: got %v", got)
	}

	if _, err := resolveVerifyTargets(repo, []string{"cobol"}); err == nil {
		t.Fatal("expected error for unknown language")
	}
}

func TestJavaTrustStoreImportsEveryCertificate(t *testing.T) {
	if _, err := exec.LookPath("keytool"); err != nil {
		t.Skip("keytool not installed")
	}
	src, err := os.ReadFile("/etc/ssl/certs/ca-certificates.crt")
	if err != nil {
		t.Skip("no system CA bundle")
	}
	// Take the first three certificates.
	var bundle []byte
	rest := src
	for i := 0; i < 3; i++ {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			t.Skip("not enough certificates")
		}
		bundle = append(bundle, pem.EncodeToMemory(b)...)
	}
	path := filepath.Join(t.TempDir(), "bundle.pem")
	if err := os.WriteFile(path, bundle, 0644); err != nil {
		t.Fatal(err)
	}
	store, err := javaTrustStore(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("keytool", "-list", "-keystore", store, "-storepass", "changeit", "-storetype", "PKCS12").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if n := strings.Count(string(out), "trustedCertEntry"); n != 3 {
		t.Fatalf("expected 3 trusted certs, got %d:\n%s", n, out)
	}
}
