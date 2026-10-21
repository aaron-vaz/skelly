package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// release is a fake of the GitHub releases pages that install.sh downloads from.
type release struct {
	server   *httptest.Server
	requests atomic.Int32
}

// newRelease serves files under /download/<tag>/ and, when latest is set, redirects /latest to
// that tag the way GitHub does.
func newRelease(t *testing.T, tag, latest string, files map[string][]byte) *release {
	t.Helper()

	r := &release{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		r.requests.Add(1)
		switch {
		case req.URL.Path == "/latest" && latest != "":
			http.Redirect(w, req, "/tag/"+latest, http.StatusFound)
		case strings.HasPrefix(req.URL.Path, "/tag/"):
			_, _ = w.Write([]byte("release page"))
		case strings.HasPrefix(req.URL.Path, "/download/"+tag+"/"):
			content, ok := files[strings.TrimPrefix(req.URL.Path, "/download/"+tag+"/")]
			if !ok {
				http.NotFound(w, req)
				return
			}
			_, _ = w.Write(content)
		default:
			http.NotFound(w, req)
		}
	})
	r.server = httptest.NewServer(mux)
	t.Cleanup(r.server.Close)
	return r
}

// hostAsset is the name of the binary that install.sh looks for on this machine.
func hostAsset(t *testing.T) string {
	t.Helper()
	if (runtime.GOOS != "linux" && runtime.GOOS != "darwin") || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		t.Skipf("skelly is not released for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is required by install.sh")
	}
	return fmt.Sprintf("skelly_%s_%s", runtime.GOOS, runtime.GOARCH)
}

func fakeBinary(tag string) []byte {
	return []byte("#!/bin/sh\necho \"skelly version " + tag + "\"\n")
}

func sha256Hex(content []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(content))
}

// checksumsFor lists files in the format of sha256sum.
func checksumsFor(files map[string][]byte, names ...string) []byte {
	var out bytes.Buffer
	for _, name := range names {
		fmt.Fprintf(&out, "%s  %s\n", sha256Hex(files[name]), name)
	}
	return out.Bytes()
}

// goodRelease is a release whose binary for this machine matches its checksums.txt.
func goodRelease(t *testing.T, tag, latest string) (*release, string) {
	t.Helper()
	asset := hostAsset(t)
	files := map[string][]byte{asset: fakeBinary(tag), "skelly_other_platform": []byte("other")}
	files["checksums.txt"] = checksumsFor(files, "skelly_other_platform", asset)
	return newRelease(t, tag, latest, files), asset
}

// stubTools puts fake commands first in the PATH of the script.
func stubTools(t *testing.T, tools map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range tools {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

type installOptions struct {
	baseURL string
	env     []string
	stubs   map[string]string
	// script is run through stdin, as curl | sh does, instead of as a file
	script string
}

// runInstall runs install.sh with a home directory of its own and a timeout, so a regression
// fails instead of hanging or touching the real one.
func runInstall(t *testing.T, opts installOptions, args ...string) skellyResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scriptPath, err := filepath.Abs(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmdArgs := append([]string{scriptPath}, args...)
	if opts.script != "" {
		cmdArgs = append([]string{"-s", "--"}, args...)
	}
	cmd := exec.CommandContext(ctx, "sh", cmdArgs...)

	env := os.Environ()
	if len(opts.stubs) > 0 {
		env = append(env, "PATH="+stubTools(t, opts.stubs)+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	env = append(env, "HOME="+t.TempDir(), "SKELLY_VERSION=", "SKELLY_INSTALL_DIR=", "SKELLY_BASE_URL="+opts.baseURL)
	cmd.Env = append(env, opts.env...)
	if opts.script != "" {
		cmd.Stdin = strings.NewReader(opts.script)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()

	return skellyResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func TestIntegration_Install_LatestRelease(t *testing.T) {
	// Given a release that is the latest one and has matching checksums
	rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
	binDir := filepath.Join(t.TempDir(), "bin")

	// When install.sh is run
	result := runInstall(t, installOptions{baseURL: rel.server.URL}, "-b", binDir)

	// Then the binary is installed and executable
	if result.err != nil {
		t.Fatalf("err = %v, stderr: %s", result.err, result.stderr)
	}
	info, err := os.Stat(filepath.Join(binDir, "skelly"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("expected an executable skelly, got %v (%v)", info, err)
	}

	// And the script says what it verified and runs the installed binary
	for _, want := range []string{"Installing skelly v1.2.3", "Checksum verified.", "skelly version v1.2.3"} {
		if !strings.Contains(result.stdout, want) {
			t.Errorf("stdout is missing %q: %s", want, result.stdout)
		}
	}
}

func TestIntegration_Install_PipedFromCurl(t *testing.T) {
	// Given the script arrives on stdin, as in curl | sh -s -- -b dir
	rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
	binDir := filepath.Join(t.TempDir(), "bin")
	script, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}

	// When it is run
	result := runInstall(t, installOptions{baseURL: rel.server.URL, script: string(script)}, "-b", binDir)

	// Then it installs the same way
	if result.err != nil {
		t.Fatalf("err = %v, stderr: %s", result.err, result.stderr)
	}
	assertExists(t, binDir, "skelly")
}

func TestIntegration_Install_TruncatedScriptRunsNothing(t *testing.T) {
	// Given a download that stopped half way through the script
	rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
	binDir := filepath.Join(t.TempDir(), "bin")
	script, err := os.ReadFile(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cut := strings.LastIndex(string(script[:len(script)*7/10]), "\n")

	// When it is run
	result := runInstall(t, installOptions{baseURL: rel.server.URL, script: string(script[:cut])}, "-b", binDir)

	// Then it does not install, or even reach out to the network
	if result.err == nil {
		t.Error("expected a truncated script to fail")
	}
	assertAbsent(t, binDir, "skelly")
	if got := rel.requests.Load(); got != 0 {
		t.Errorf("expected no requests, got %d", got)
	}
}

func TestIntegration_Install_PinnedVersion(t *testing.T) {
	// Given a release that is not the latest one
	rel, _ := goodRelease(t, "v1.0.0", "")
	binDir := filepath.Join(t.TempDir(), "bin")

	// When a version is asked for, without the leading v
	result := runInstall(t, installOptions{baseURL: rel.server.URL}, "-v", "1.0.0", "-b", binDir)

	// Then that version is installed
	if result.err != nil {
		t.Fatalf("err = %v, stderr: %s", result.err, result.stderr)
	}
	if !strings.Contains(result.stdout, "skelly version v1.0.0") {
		t.Errorf("expected v1.0.0 to be installed, got %s", result.stdout)
	}
}

func TestIntegration_Install_RefusesWhatItCannotVerify(t *testing.T) {
	asset := hostAsset(t)
	binary := fakeBinary("v1.2.3")
	other := []byte("other")

	tests := []struct {
		name  string
		files map[string][]byte
		want  string
	}{
		{
			name: "a binary that does not match its checksum",
			files: map[string][]byte{
				asset:           binary,
				"checksums.txt": []byte(sha256Hex([]byte("something else")) + "  " + asset + "\n"),
			},
			want: "does not match checksums.txt",
		},
		{
			name:  "a release without checksums",
			files: map[string][]byte{asset: binary},
			want:  "could not download",
		},
		{
			name: "checksums that leave out this platform",
			files: map[string][]byte{
				asset:           binary,
				"checksums.txt": checksumsFor(map[string][]byte{"skelly_other_platform": other}, "skelly_other_platform"),
			},
			want: "no entry for " + asset,
		},
		{
			name: "two entries for the same file",
			files: map[string][]byte{
				asset: binary,
				"checksums.txt": []byte(
					sha256Hex(binary) + "  " + asset + "\n" + sha256Hex(other) + "  " + asset + "\n"),
			},
			want: "more than one entry",
		},
		{
			name: "a checksum that is not a hash",
			files: map[string][]byte{
				asset:           binary,
				"checksums.txt": []byte("not-a-hash  " + asset + "\n"),
			},
			want: "not a SHA-256 hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a release that cannot be verified
			rel := newRelease(t, "v1.2.3", "v1.2.3", tt.files)
			binDir := filepath.Join(t.TempDir(), "bin")

			// When install.sh is run
			result := runInstall(t, installOptions{baseURL: rel.server.URL}, "-b", binDir)

			// Then it fails, says why, and installs nothing
			if result.err == nil {
				t.Fatalf("expected a failure, stdout: %s", result.stdout)
			}
			if !strings.Contains(result.stderr, tt.want) {
				t.Errorf("stderr = %q, want it to contain %q", result.stderr, tt.want)
			}
			assertAbsent(t, binDir, "skelly")
		})
	}
}

func TestIntegration_Install_RejectsBadVersions(t *testing.T) {
	for _, version := range []string{"v1;touch pwned", "../../etc", "v1.2", "latest", "v1.2.3/../x", "$(id)", "v1.2.3\nx"} {
		t.Run(strings.ReplaceAll(version, "\n", "_newline_"), func(t *testing.T) {
			// Given a server that must not be reached
			rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
			binDir := filepath.Join(t.TempDir(), "bin")

			// When a version that is not a release is asked for
			result := runInstall(t, installOptions{baseURL: rel.server.URL}, "-v", version, "-b", binDir)

			// Then it is refused before anything is downloaded
			if result.err == nil || !strings.Contains(result.stderr, "not a release version") {
				t.Errorf("err = %v, stderr = %q", result.err, result.stderr)
			}
			if got := rel.requests.Load(); got != 0 {
				t.Errorf("expected no requests, got %d", got)
			}
			assertAbsent(t, binDir, "skelly")
		})
	}
}

func TestIntegration_Install_OnlyHTTPS(t *testing.T) {
	for _, base := range []string{
		"http://example.com/releases",
		"ftp://example.com",
		"http://localhost.evil.example",
		// curl reads these as a login on the host after the @
		"http://localhost:80@evil.example/",
		"http://127.0.0.1:x@evil.example",
		"https://user:secret@example.com",
		"http://localhost:80x",
	} {
		t.Run(base, func(t *testing.T) {
			// Given a base URL that is not https and not this machine
			binDir := filepath.Join(t.TempDir(), "bin")

			// When install.sh is run
			result := runInstall(t, installOptions{baseURL: base}, "-b", binDir)

			// Then it is refused
			refused := strings.Contains(result.stderr, "must be an https URL") || strings.Contains(result.stderr, "must not contain credentials")
			if result.err == nil || !refused {
				t.Errorf("err = %v, stderr = %q", result.err, result.stderr)
			}
			assertAbsent(t, binDir, "skelly")
		})
	}
}

func TestIntegration_Install_UnsupportedPlatform(t *testing.T) {
	tests := []struct {
		name string
		os   string
		arch string
		want string
	}{
		{"operating system", "FreeBSD", "x86_64", "unsupported operating system"},
		{"architecture", "Linux", "riscv64", "unsupported architecture"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a machine skelly is not released for
			rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
			binDir := filepath.Join(t.TempDir(), "bin")
			uname := fmt.Sprintf("case \"$1\" in -s) echo %s ;; -m) echo %s ;; esac\n", tt.os, tt.arch)

			// When install.sh is run
			result := runInstall(t, installOptions{baseURL: rel.server.URL, stubs: map[string]string{"uname": uname}}, "-b", binDir)

			// Then it says so and installs nothing
			if result.err == nil || !strings.Contains(result.stderr, tt.want) {
				t.Errorf("err = %v, stderr = %q", result.err, result.stderr)
			}
			assertAbsent(t, binDir, "skelly")
		})
	}
}

func TestIntegration_Install_ReplacesAnExistingBinary(t *testing.T) {
	// Given skelly is already installed
	rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "skelly"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	// When install.sh is run
	result := runInstall(t, installOptions{baseURL: rel.server.URL}, "-b", binDir)

	// Then the binary is replaced, and no staging file is left behind
	if result.err != nil {
		t.Fatalf("err = %v, stderr: %s", result.err, result.stderr)
	}
	assertContent(t, filepath.Join(binDir, "skelly"), string(fakeBinary("v1.2.3")))
	entries, err := os.ReadDir(binDir)
	if err != nil || len(entries) != 1 {
		t.Errorf("expected only skelly in %s, got %v (%v)", binDir, entries, err)
	}
}

func TestIntegration_Install_FailedChecksumKeepsTheOldBinary(t *testing.T) {
	// Given skelly is installed and the new release does not match its checksum
	asset := hostAsset(t)
	rel := newRelease(t, "v1.2.3", "v1.2.3", map[string][]byte{
		asset:           fakeBinary("v1.2.3"),
		"checksums.txt": []byte(sha256Hex([]byte("tampered")) + "  " + asset + "\n"),
	})
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "skelly"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	// When install.sh is run
	result := runInstall(t, installOptions{baseURL: rel.server.URL}, "-b", binDir)

	// Then the old binary is untouched
	if result.err == nil {
		t.Fatal("expected a failure")
	}
	assertContent(t, filepath.Join(binDir, "skelly"), "old")
}

func TestIntegration_Install_Provenance(t *testing.T) {
	t.Run("is checked with gh before installing", func(t *testing.T) {
		// Given a gh that accepts the attestation, and records how it was called
		rel, asset := goodRelease(t, "v1.2.3", "v1.2.3")
		binDir := filepath.Join(t.TempDir(), "bin")
		calls := filepath.Join(t.TempDir(), "gh-calls")
		gh := fmt.Sprintf("echo \"$@\" > %q\nexit 0\n", calls)

		// When install.sh is asked to verify the provenance
		result := runInstall(t, installOptions{baseURL: rel.server.URL, stubs: map[string]string{"gh": gh}}, "-p", "-b", binDir)

		// Then gh was asked to verify the downloaded binary against this repository
		if result.err != nil {
			t.Fatalf("err = %v, stderr: %s", result.err, result.stderr)
		}
		recorded, err := os.ReadFile(calls)
		if err != nil {
			t.Fatalf("gh was not called: %v", err)
		}
		// And it has to be the release workflow of this repository, run for this tag
		for _, want := range []string{
			"attestation verify", asset, "--repo aaron-vaz/skelly",
			"--signer-workflow aaron-vaz/skelly/.github/workflows/release.yml",
			"--source-ref refs/tags/v1.2.3",
		} {
			if !strings.Contains(string(recorded), want) {
				t.Errorf("gh was called with %q, want it to contain %q", recorded, want)
			}
		}

		// And the binary is installed
		assertExists(t, binDir, "skelly")
	})

	t.Run("that fails stops the install", func(t *testing.T) {
		// Given a gh that rejects the attestation
		rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
		binDir := filepath.Join(t.TempDir(), "bin")

		// When install.sh is asked to verify the provenance
		result := runInstall(t, installOptions{baseURL: rel.server.URL, stubs: map[string]string{"gh": "exit 1\n"}}, "--verify-provenance", "-b", binDir)

		// Then nothing is installed, although the checksum matched
		if result.err == nil || !strings.Contains(result.stderr, "attestation") {
			t.Errorf("err = %v, stderr = %q", result.err, result.stderr)
		}
		assertAbsent(t, binDir, "skelly")
	})
}

func TestIntegration_Install_Help(t *testing.T) {
	// When
	result := runInstall(t, installOptions{}, "--help")

	// Then
	if result.err != nil || !strings.Contains(result.stdout, "--verify-provenance") {
		t.Errorf("err = %v, stdout = %q", result.err, result.stdout)
	}
}

func TestIntegration_Install_SharedDirectory(t *testing.T) {
	tests := []struct {
		name    string
		mode    os.FileMode
		refused bool
	}{
		{"writable by everyone is refused", 0o777, true},
		{"writable by everyone but sticky, like /tmp, is fine", 0o777 | os.ModeSticky, false},
		{"private is fine", 0o700, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a destination with those permissions
			rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
			binDir := t.TempDir()
			if err := os.Chmod(binDir, tt.mode); err != nil {
				t.Fatal(err)
			}

			// When install.sh is run
			result := runInstall(t, installOptions{baseURL: rel.server.URL}, "-b", binDir)

			// Then other users being able to rename files in it stops the install
			if tt.refused {
				if result.err == nil || !strings.Contains(result.stderr, "other users") {
					t.Errorf("err = %v, stderr = %q", result.err, result.stderr)
				}
				assertAbsent(t, binDir, "skelly")
				return
			}
			if result.err != nil {
				t.Fatalf("err = %v, stderr: %s", result.err, result.stderr)
			}
			assertExists(t, binDir, "skelly")
		})
	}
}

func TestIntegration_Install_DestinationIsADirectory(t *testing.T) {
	// Given a directory where the binary should go, which mv would move the new binary into
	rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
	binDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(binDir, "skelly"), 0o755); err != nil {
		t.Fatal(err)
	}

	// When install.sh is run
	result := runInstall(t, installOptions{baseURL: rel.server.URL}, "-b", binDir)

	// Then it refuses and nothing is put inside it
	if result.err == nil || !strings.Contains(result.stderr, "is a directory") {
		t.Errorf("err = %v, stderr = %q", result.err, result.stderr)
	}
	entries, _ := os.ReadDir(filepath.Join(binDir, "skelly"))
	if len(entries) != 0 {
		t.Errorf("expected nothing inside the directory, got %v", entries)
	}
}

func TestIntegration_Install_TrailingSlashAndPATH(t *testing.T) {
	// Given a directory that is in the PATH, given with a trailing slash
	rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
	binDir := t.TempDir()

	// When install.sh is run
	result := runInstall(t, installOptions{
		baseURL: rel.server.URL,
		env:     []string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")},
	}, "-b", binDir+"/")

	// Then it does not claim the directory is missing from the PATH
	if result.err != nil {
		t.Fatalf("err = %v, stderr: %s", result.err, result.stderr)
	}
	if strings.Contains(result.stdout, "not in your PATH") {
		t.Errorf("unexpected PATH note: %s", result.stdout)
	}
}

func TestIntegration_Install_StagesWithAnUnpredictableName(t *testing.T) {
	if _, err := os.Stat("/usr/bin/mktemp"); err != nil {
		t.Skip("needs /usr/bin/mktemp to wrap it")
	}

	// Given a mktemp that records how it is called
	rel, _ := goodRelease(t, "v1.2.3", "v1.2.3")
	binDir := t.TempDir()
	calls := filepath.Join(t.TempDir(), "mktemp-calls")
	mktemp := fmt.Sprintf("echo \"$*\" >> %q\nexec /usr/bin/mktemp \"$@\"\n", calls)

	// When install.sh is run
	result := runInstall(t, installOptions{baseURL: rel.server.URL, stubs: map[string]string{"mktemp": mktemp}}, "-b", binDir)

	// Then the staged file is made in the destination by mktemp, so its name cannot be guessed
	// and a link planted in a shared directory is not written through
	if result.err != nil {
		t.Fatalf("err = %v, stderr: %s", result.err, result.stderr)
	}
	recorded, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(binDir, ".skelly.XXXXXX"); !strings.Contains(string(recorded), want) {
		t.Errorf("mktemp was called with %q, want a call for %q", recorded, want)
	}

	// And no staging file is left behind
	entries, err := os.ReadDir(binDir)
	if err != nil || len(entries) != 1 {
		t.Errorf("expected only skelly in %s, got %v (%v)", binDir, entries, err)
	}
}
