// Package updater implements self-update: it checks the project's GitHub
// releases and, when a newer version exists, downloads the matching archive and
// replaces the running executable in place.
package updater

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const repoSlug = "okyashgajjar/costaffective-mcp"

// ErrUpToDate is returned by Update when no newer release is available.
var ErrUpToDate = errors.New("already up to date")

// Release is the subset of the GitHub release API we use.
type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func client() *http.Client { return &http.Client{Timeout: 60 * time.Second} }

// FetchLatest returns the latest published release.
func FetchLatest() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoSlug)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %s", resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// IsNewer reports whether latest is a higher semantic version than current.
// A non-semver current (e.g. "dev") is treated as older, so updates are offered.
func IsNewer(latest, current string) bool {
	lc, cc := parseSemver(latest), parseSemver(current)
	if cc == nil {
		return lc != nil
	}
	if lc == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if lc[i] != cc[i] {
			return lc[i] > cc[i]
		}
	}
	return false
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	v = strings.SplitN(v, "-", 2)[0] // drop -SNAPSHOT / prerelease suffix
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return nil
	}
	out := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}

// assetPrefix matches the goreleaser name_template costaffective_<os>_<arch>.
func assetPrefix() string {
	return fmt.Sprintf("costaffective_%s_%s", runtime.GOOS, runtime.GOARCH)
}

// Update fetches the latest release and, if newer than current, downloads the
// platform archive and replaces the running binary. Returns the version it
// installed, or ErrUpToDate.
func Update(current string) (string, error) {
	rel, err := FetchLatest()
	if err != nil {
		return "", err
	}
	if !IsNewer(rel.TagName, current) {
		return rel.TagName, ErrUpToDate
	}

	prefix := assetPrefix()
	var dlURL string
	for _, a := range rel.Assets {
		if strings.HasPrefix(a.Name, prefix) && strings.HasSuffix(a.Name, ".zip") {
			dlURL = a.BrowserDownloadURL
			break
		}
	}
	if dlURL == "" {
		return "", fmt.Errorf("release %s has no asset for %s/%s", rel.TagName, runtime.GOOS, runtime.GOARCH)
	}

	archive, err := download(dlURL)
	if err != nil {
		return "", err
	}
	binName := "costaffective"
	if runtime.GOOS == "windows" {
		binName = "costaffective.exe"
	}
	binData, err := extractFromZip(archive, binName)
	if err != nil {
		return "", err
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if err := replaceExecutable(exe, binData); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

func download(url string) ([]byte, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	resp, err := client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func extractFromZip(data []byte, binName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) == binName {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("binary %q not found in release archive", binName)
}

// replaceExecutable writes data to a temp file next to path and renames it over
// path. On Windows the running .exe is moved aside first (it cannot be
// overwritten while executing).
func replaceExecutable(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".costaffective-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s (try with elevated permissions): %w", dir, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	tmp.Close()
	if err := os.Chmod(tmpName, 0o755); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	if runtime.GOOS == "windows" {
		old := path + ".old"
		_ = os.Remove(old)
		if err := os.Rename(path, old); err != nil {
			_ = os.Remove(tmpName)
			return err
		}
		if err := os.Rename(tmpName, path); err != nil {
			_ = os.Rename(old, path) // rollback
			_ = os.Remove(tmpName)
			return err
		}
		return nil
	}
	return os.Rename(tmpName, path)
}
