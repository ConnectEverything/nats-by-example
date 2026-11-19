package main

import (
	"bufio"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Support for building images behind a TLS-intercepting proxy (common on
// corporate networks and in sandboxed CI). If NBE_CA_BUNDLE points to a PEM
// bundle (which must include the public roots as well as the proxy CA), the
// bundle is copied into the build context and every non-scratch build stage
// is configured to trust it. Without the variable, images are built unchanged.

const caBundleEnv = "NBE_CA_BUNDLE"

var (
	javaTrustOnce sync.Once
	javaTrustPath string
	javaTrustErr  error
)

// javaTrustStore converts the PEM bundle into a PKCS12 trust store once per
// process. keytool imports only the first certificate of a PEM file, so each
// certificate is imported separately.
func javaTrustStore(bundle string) (string, error) {
	javaTrustOnce.Do(func() {
		if _, err := exec.LookPath("keytool"); err != nil {
			javaTrustErr = err
			return
		}
		data, err := os.ReadFile(bundle)
		if err != nil {
			javaTrustErr = err
			return
		}
		dir, err := os.MkdirTemp("", "nbe-ca")
		if err != nil {
			javaTrustErr = err
			return
		}
		store := filepath.Join(dir, "cacerts.p12")
		n := 0
		for {
			var block *pem.Block
			block, data = pem.Decode(data)
			if block == nil {
				break
			}
			if block.Type != "CERTIFICATE" {
				continue
			}
			certFile := filepath.Join(dir, "cert.pem")
			if err := os.WriteFile(certFile, pem.EncodeToMemory(block), 0644); err != nil {
				javaTrustErr = err
				return
			}
			cmd := exec.Command("keytool", "-importcert", "-noprompt",
				"-alias", fmt.Sprintf("nbe-ca-%d", n),
				"-file", certFile,
				"-keystore", store,
				"-storetype", "PKCS12",
				"-storepass", "changeit")
			if out, err := cmd.CombinedOutput(); err != nil {
				javaTrustErr = fmt.Errorf("keytool: %w: %s", err, out)
				return
			}
			n++
		}
		if n == 0 {
			javaTrustErr = fmt.Errorf("no certificates in %s", bundle)
			return
		}
		javaTrustPath = store
	})
	return javaTrustPath, javaTrustErr
}

var dockerFromRe = regexp.MustCompile(`(?i)^\s*FROM\s+(?:--platform=\S+\s+)?(\S+)`)

func injectCABundle(buildDir string) error {
	bundle := os.Getenv(caBundleEnv)
	if bundle == "" {
		return nil
	}

	dockerfile := filepath.Join(buildDir, "Dockerfile")
	if _, err := os.Stat(dockerfile); err != nil {
		return nil
	}

	caDir := filepath.Join(buildDir, ".nbe-ca")
	if err := os.MkdirAll(caDir, 0755); err != nil {
		return err
	}
	if err := copyFile(bundle, filepath.Join(caDir, "ca-bundle.crt")); err != nil {
		return fmt.Errorf("%s: %w", caBundleEnv, err)
	}

	// Java needs a keystore rather than PEM. Best effort if keytool exists.
	javaTrust := false
	if ks, err := javaTrustStore(bundle); err == nil {
		javaTrust = copyFile(ks, filepath.Join(caDir, "cacerts.p12")) == nil
	}

	f, err := os.Open(dockerfile)
	if err != nil {
		return err
	}
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	f.Close()
	if err := sc.Err(); err != nil {
		return err
	}

	lastFrom := -1
	for i, l := range lines {
		if dockerFromRe.MatchString(l) {
			lastFrom = i
		}
	}

	const ca = "/nbe-ca/ca-bundle.crt"
	envs := []string{
		"SSL_CERT_FILE=" + ca,
		"REQUESTS_CA_BUNDLE=" + ca,
		"PIP_CERT=" + ca,
		"NODE_EXTRA_CA_CERTS=" + ca,
		"CARGO_HTTP_CAINFO=" + ca,
		"CURL_CA_BUNDLE=" + ca,
		"GIT_SSL_CAINFO=" + ca,
		"DENO_CERT=" + ca,
		"HEX_CACERTS_PATH=" + ca,
	}

	var out []string
	for i, l := range lines {
		out = append(out, l)
		m := dockerFromRe.FindStringSubmatch(l)
		if m == nil || strings.EqualFold(m[1], "scratch") {
			continue
		}
		out = append(out,
			"COPY .nbe-ca/ /nbe-ca/",
			"COPY .nbe-ca/ca-bundle.crt /etc/ssl/certs/ca-certificates.crt",
			// Picked up again if the build (re)installs ca-certificates,
			// which regenerates the file above.
			"COPY .nbe-ca/ca-bundle.crt /usr/local/share/ca-certificates/nbe-ca.crt",
			"ENV "+strings.Join(envs, " "),
		)
		// JAVA_TOOL_OPTIONS makes the JVM print a notice, so only set it in
		// build stages, not in the stage that runs the example.
		if javaTrust && i != lastFrom {
			out = append(out, `ENV JAVA_TOOL_OPTIONS="-Djavax.net.ssl.trustStore=/nbe-ca/cacerts.p12 -Djavax.net.ssl.trustStorePassword=changeit -Djavax.net.ssl.trustStoreType=PKCS12"`)
		}
	}

	return os.WriteFile(dockerfile, []byte(strings.Join(out, "\n")+"\n"), 0644)
}
