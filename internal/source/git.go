package source

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// CloneGit clones repoURL into destDir (shallow, single branch). If token is
// non-empty it is embedded in the URL as a credential for HTTPS remotes.
func CloneGit(repoURL, destDir, token string) error {
	cloneURL := repoURL
	if token != "" {
		cloneURL = embedToken(repoURL, token)
	}
	cmd := exec.Command("git", "clone", "--depth=1", "--quiet", cloneURL, destDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone %s: %w\n%s", repoURL, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func embedToken(rawURL, token string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" {
		return rawURL
	}
	u.User = url.UserPassword("oauth2", token)
	return u.String()
}
