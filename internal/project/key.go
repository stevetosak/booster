package project

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// The kalfa project key names a repository's layer in kalfa's Memory home:
// host/owner/repo from the origin remote, else local/<folder>. KeyFromRemote
// and Key follow keyFromRemote and projectKey in kalfa's
// src/core/memory/home.ts and must give the same key for every checkout.

var (
	scpRemote  = regexp.MustCompile(`^(?:[^@/:]+@)?([^:/]+):(.+)$`)
	urlScheme  = regexp.MustCompile(`(?i)^[a-z][a-z0-9+.-]*://`)
	gitSuffix  = regexp.MustCompile(`\.git/?$`)
	keySegment = regexp.MustCompile(`^[a-z0-9._-]+$`)
	notKeyRuns = regexp.MustCompile(`[^a-z0-9._-]+`)
)

// KeyFromRemote returns host/owner/repo from a git remote URL (scp-like,
// ssh:// or https://), or "" when the URL has no such shape.
//
// kalfa parses a URL with the WHATWG parser of Node and this with net/url;
// they differ on a URL whose path holds "..", an IPv6 host (kalfa keeps the
// brackets) and a non-ASCII host (kalfa makes it punycode). No remote of the
// Operator's has one of these shapes.
func KeyFromRemote(remote string) string {
	host, path := splitRemote(remote)
	if host == "" {
		return ""
	}
	var segments []string
	for _, s := range strings.Split(gitSuffix.ReplaceAllString(path, ""), "/") {
		if s == "" {
			continue
		}
		s = strings.ToLower(s)
		if s == "." || s == ".." || !keySegment.MatchString(s) {
			return ""
		}
		segments = append(segments, s)
	}
	if len(segments) < 2 {
		return ""
	}
	return strings.ToLower(host) + "/" + strings.Join(segments, "/")
}

// splitRemote returns the host and the path of a remote: the scp-like form
// host:path when its path does not start with a slash and the URL has no
// scheme, else the URL's host and its path as written (still escaped).
func splitRemote(remote string) (host, path string) {
	if m := scpRemote.FindStringSubmatch(remote); m != nil && !strings.HasPrefix(m[2], "/") && !urlScheme.MatchString(remote) {
		return m[1], m[2]
	}
	u, err := url.Parse(remote)
	if err != nil {
		return "", ""
	}
	return u.Hostname(), u.EscapedPath()
}

// Key returns the kalfa project key of the checkout at dir: from what
// `git remote get-url origin` prints there, else local/ and the folder name.
func Key(dir string) string {
	if out, err := git(dir, "remote", "get-url", "origin"); err == nil {
		if key := KeyFromRemote(strings.TrimSpace(out)); key != "" {
			return key
		}
	}
	return "local/" + notKeyRuns.ReplaceAllString(strings.ToLower(filepath.Base(dir)), "-")
}
