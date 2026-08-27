package source

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// CloneGit clones repoURL into destDir (shallow, single branch). If token is
// non-empty it is embedded in the URL as a credential for HTTPS remotes.
// If branch is non-empty only that branch is cloned.
func CloneGit(repoURL, destDir, token, branch string) error {
	cloneURL := repoURL
	if token != "" {
		cloneURL = embedToken(repoURL, token)
	}
	args := []string{"clone", "--depth=1", "--quiet"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, cloneURL, destDir)
	cmd := exec.Command("git", args...)
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
