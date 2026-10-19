//go:build linux

// Package updater manages release update checks and binary self-upgrades for swapaudit.
package updater

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

const (
	// DefaultRepo is the canonical GitHub repository for swapaudit.
	DefaultRepo = "jadmadi/swapaudit"

	// CheckInterval is the minimum interval between automated release checks.
	CheckInterval = 24 * time.Hour

	// RequestTimeout is the maximum duration to wait for remote release checks.
	RequestTimeout = 2 * time.Second
)

// State holds the persistent timestamp and version of the last release check.
type State struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
	ReleaseURL    string    `json:"release_url,omitempty"`
}

// UpdateInfo contains details about an available update.
type UpdateInfo struct {
	Current    string
	Latest     string
	ReleaseURL string
}

// CacheFilePath returns the path to the persistent update check state file.
func CacheFilePath() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "swapaudit", "update_check.json")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "swapaudit", "update_check.json")
	}
	return filepath.Join(os.TempDir(), "swapaudit-update-check.json")
}

// LoadState reads the update state from the specified path.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveState writes the update state atomically to the specified path.
func SaveState(path string, s *State) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// cleanVersion strips prefix 'v' or 'V' and any metadata or dirty suffixes.
func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// parseSemVer parses a version string into (major, minor, patch, prerelease).
func parseSemVer(v string) (int, int, int, string, bool) {
	v = cleanVersion(v)
	if v == "" || v == "dev" || v == "none" || strings.HasPrefix(v, "dev+") {
		return 0, 0, 0, v, false
	}

	prerelease := ""
	if idx := strings.Index(v, "-"); idx >= 0 {
		prerelease = v[idx+1:]
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, 0, prerelease, false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, prerelease, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, prerelease, false
	}
	patch := 0
	if len(parts) >= 3 {
		p, err := strconv.Atoi(parts[2])
		if err == nil {
			patch = p
		}
	}

	return major, minor, patch, prerelease, true
}

// CompareVersions compares two semantic version strings.
// Returns -1 if v1 < v2, 0 if v1 == v2, and 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	maj1, min1, pat1, pre1, ok1 := parseSemVer(v1)
	maj2, min2, pat2, pre2, ok2 := parseSemVer(v2)

	if !ok1 && !ok2 {
		return strings.Compare(cleanVersion(v1), cleanVersion(v2))
	}
	if !ok1 {
		return -1
	}
	if !ok2 {
		return 1
	}

	if maj1 != maj2 {
		if maj1 < maj2 {
			return -1
		}
		return 1
	}
	if min1 != min2 {
		if min1 < min2 {
			return -1
		}
		return 1
	}
	if pat1 != pat2 {
		if pat1 < pat2 {
			return -1
		}
		return 1
	}

	// Normal release is newer than pre-release of the same version
	if pre1 != "" && pre2 == "" {
		return -1
	}
	if pre1 == "" && pre2 != "" {
		return 1
	}
	return strings.Compare(pre1, pre2)
}

// IsNewer returns true if target represents a newer release than current.
func IsNewer(current, target string) bool {
	if target == "" {
		return false
	}
	_, _, _, _, okTarget := parseSemVer(target)
	if !okTarget {
		return false
	}

	_, _, _, _, okCurrent := parseSemVer(current)
	if !okCurrent {
		// Non-semver builds like "dev" or "none" are superseded by real releases
		return true
	}

	return CompareVersions(current, target) < 0
}

// FetchLatestRelease queries GitHub for the latest tag of the specified repository.
func FetchLatestRelease(repo string, client *http.Client) (string, string, error) {
	if client == nil {
		client = &http.Client{Timeout: RequestTimeout}
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "swapaudit")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var rel struct {
				TagName string `json:"tag_name"`
				HTMLURL string `json:"html_url"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&rel); err == nil && rel.TagName != "" {
				return rel.TagName, rel.HTMLURL, nil
			}
		}
	}

	// Fallback: check redirect header on github.com/<repo>/releases/latest
	redirectClient := &http.Client{
		Timeout: RequestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	webURL := fmt.Sprintf("https://github.com/%s/releases/latest", repo)
	webReq, err := http.NewRequest(http.MethodGet, webURL, nil)
	if err != nil {
		return "", "", err
	}
	webReq.Header.Set("User-Agent", "swapaudit")

	webResp, err := redirectClient.Do(webReq)
	if err != nil {
		return "", "", err
	}
	defer webResp.Body.Close()

	if webResp.StatusCode >= 300 && webResp.StatusCode < 400 {
		loc := webResp.Header.Get("Location")
		if idx := strings.LastIndex(loc, "/tag/"); idx >= 0 {
			tag := loc[idx+len("/tag/"):]
			if tag != "" {
				return tag, loc, nil
			}
		}
	}

	return "", "", fmt.Errorf("no release found for %s", repo)
}

// CheckDailyUpdate checks for an available release update at most once per 24 hours.
// Returns update details if a newer version is available, or nil if current or checked recently.
func CheckDailyUpdate(currentVersion string) (*UpdateInfo, error) {
	if os.Getenv("SWAPAUDIT_NO_UPDATE_CHECK") != "" || os.Getenv("CI") != "" {
		return nil, nil
	}

	path := CacheFilePath()
	now := time.Now()

	state, err := LoadState(path)
	if err == nil && state != nil && now.Sub(state.LastChecked) < CheckInterval {
		if IsNewer(currentVersion, state.LatestVersion) {
			return &UpdateInfo{
				Current:    currentVersion,
				Latest:     state.LatestVersion,
				ReleaseURL: state.ReleaseURL,
			}, nil
		}
		return nil, nil
	}

	repo := os.Getenv("SWAPAUDIT_REPO")
	if repo == "" {
		repo = DefaultRepo
	}

	latest, releaseURL, fetchErr := FetchLatestRelease(repo, nil)

	// Save check timestamp even on failure to avoid repeating network requests today
	newState := &State{
		LastChecked:   now,
		LatestVersion: latest,
		ReleaseURL:    releaseURL,
	}
	if fetchErr == nil {
		_ = SaveState(path, newState)
	} else if state != nil {
		// Retain previously known latest version on transient network error
		newState.LatestVersion = state.LatestVersion
		newState.ReleaseURL = state.ReleaseURL
		_ = SaveState(path, newState)
	} else {
		_ = SaveState(path, newState)
	}

	if fetchErr != nil {
		return nil, fetchErr
	}

	if IsNewer(currentVersion, latest) {
		return &UpdateInfo{
			Current:    currentVersion,
			Latest:     latest,
			ReleaseURL: releaseURL,
		}, nil
	}

	return nil, nil
}

// Upgrade downloads the latest release binary and atomically replaces the running executable.
func Upgrade(currentVersion string, out, errOut io.Writer) int {
	repo := os.Getenv("SWAPAUDIT_REPO")
	if repo == "" {
		repo = DefaultRepo
	}

	fmt.Fprintln(out, "Checking for latest swapaudit release...")
	client := &http.Client{Timeout: 10 * time.Second}
	latest, releaseURL, err := FetchLatestRelease(repo, client)
	if err != nil {
		fmt.Fprintf(errOut, "Error checking for updates: %v\n", err)
		fmt.Fprintf(errOut, "You can visit the releases page directly: https://github.com/%s/releases\n", repo)
		return 1
	}

	if !IsNewer(currentVersion, latest) {
		fmt.Fprintf(out, "swapaudit is already up to date (%s).\n", currentVersion)
		return 0
	}

	fmt.Fprintf(out, "Upgrading swapaudit %s -> %s...\n", currentVersion, latest)

	selfPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(errOut, "Error locating current executable: %v\n", err)
		return 1
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		fmt.Fprintf(errOut, "Error resolving executable symlinks: %v\n", err)
		return 1
	}

	targetDir := filepath.Dir(selfPath)
	if err := checkWritable(targetDir); err != nil {
		fmt.Fprintf(errOut, "Error: cannot write to install directory %s (%v).\n", targetDir, err)
		fmt.Fprintf(errOut, "Please run with elevated permissions:\n  sudo swapaudit upgrade\n")
		return 1
	}

	arch := runtime.GOARCH
	assetName := fmt.Sprintf("swapaudit-linux-%s", arch)
	assetURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, latest, assetName)
	checksumURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/checksums.txt", repo, latest)

	fmt.Fprintf(out, "Downloading %s from %s...\n", assetName, assetURL)
	tmpBin := filepath.Join(targetDir, fmt.Sprintf(".swapaudit.tmp.%d", os.Getpid()))
	defer os.Remove(tmpBin)

	if err := downloadFile(client, assetURL, tmpBin); err != nil {
		fmt.Fprintf(errOut, "Error downloading release binary: %v\n", err)
		return 1
	}

	// Verify checksum if available
	expectedChecksum := fetchChecksum(client, checksumURL, assetName)
	if expectedChecksum != "" {
		actualChecksum, err := computeSHA256(tmpBin)
		if err != nil {
			fmt.Fprintf(errOut, "Error computing binary checksum: %v\n", err)
			return 1
		}
		if !strings.EqualFold(actualChecksum, expectedChecksum) {
			fmt.Fprintf(errOut, "Checksum mismatch for %s:\n  expected: %s\n  actual:   %s\n",
				assetName, expectedChecksum, actualChecksum)
			return 1
		}
		fmt.Fprintln(out, "Checksum verified successfully.")
	}

	if err := os.Chmod(tmpBin, 0o755); err != nil {
		fmt.Fprintf(errOut, "Error setting executable permissions: %v\n", err)
		return 1
	}

	if err := os.Rename(tmpBin, selfPath); err != nil {
		fmt.Fprintf(errOut, "Error replacing executable at %s: %v\n", selfPath, err)
		return 1
	}

	// Update local state cache
	_ = SaveState(CacheFilePath(), &State{
		LastChecked:   time.Now(),
		LatestVersion: latest,
		ReleaseURL:    releaseURL,
	})

	// Also synchronize active go script directory if it exists and differs
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		activeTarget := filepath.Join(home, "scripts", "active", "go", "swapaudit")
		if activeTarget != selfPath {
			if fi, err := os.Stat(activeTarget); err == nil && !fi.IsDir() {
				if copyErr := copyFileAtomic(selfPath, activeTarget); copyErr == nil {
					fmt.Fprintf(out, "Synchronized active go binary at %s.\n", activeTarget)
				}
			}
		}
	}

	fmt.Fprintf(out, "✔ Successfully upgraded swapaudit to %s at %s.\n", latest, selfPath)
	return 0
}

func checkWritable(dir string) error {
	testFile := filepath.Join(dir, fmt.Sprintf(".test_write.%d", os.Getpid()))
	if err := os.WriteFile(testFile, []byte(""), 0o644); err != nil {
		return err
	}
	_ = os.Remove(testFile)
	return nil
}

func downloadFile(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func fetchChecksum(client *http.Client, checksumURL, assetName string) string {
	resp, err := client.Get(checksumURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			fileName := filepath.Base(fields[1])
			if fileName == assetName {
				return fields[0]
			}
		}
	}
	return ""
}

func computeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFileAtomic(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := fmt.Sprintf("%s.tmp.%d", dest, os.Getpid())
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	out.Close()
	return os.Rename(tmp, dest)
}
