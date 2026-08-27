package source

import (
	"fmt"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// CloneGit clones repoURL into destDir (shallow, single branch) using pure Go.
// If token is non-empty it is used as a Bearer token for HTTPS remotes.
// If branch is non-empty only that branch is cloned.
func CloneGit(repoURL, destDir, token, branch string) error {
	opts := &gogit.CloneOptions{
		URL:             repoURL,
		Depth:           1,
		InsecureSkipTLS: true, //nolint:gosec
	}
	if token != "" {
		opts.Auth = &http.BasicAuth{Username: "oauth2", Password: token}
	}
	if branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		opts.SingleBranch = true
	}
	_, err := gogit.PlainClone(destDir, false, opts)
	if err != nil {
		return fmt.Errorf("git clone %s: %w", repoURL, err)
	}
	return nil
}
