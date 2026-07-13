package gitverify

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/supply-chain-tools/go-sandbox/gitkit"
)

type SignatureType string

const (
	SignatureTypePGP         SignatureType = "pgp"
	SignatureTypeSSH         SignatureType = "ssh"
	SignatureTypeNone        SignatureType = "none"
	SignatureTypeSMime       SignatureType = "smime"
	SignatureTypeUnknown     SignatureType = "unknown"
	sshExpectedNamespace     string        = "git"
	sshExpectedReservedField string        = ""
)

var sshExpectedMagicPreamble = []byte("SSHSIG")

type CommitData struct {
	AfterOrAncestorOfAfter bool
	ConnectedToAfter       bool
	SignatureVerified      bool
	MergeTag               *object.Tag
}

func InferForgeOrgAndRepo(repo *git.Repository) (forge string, org string, repoName string, err error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", "", "", err
	}
	urls := remote.Config().URLs
	if len(urls) != 1 {
		return "", "", "", fmt.Errorf("expected exactly one remote url, got %d", len(urls))
	}

	repoUrl := urls[0]
	forge = gitHubForgeId
	org, repoName, err = getGitHubOrgRepo(forge, repoUrl)
	if err != nil {
		forge = gitlabForgeId
		org, repoName, err = getGitHubOrgRepo(forge, repoUrl)
		if err != nil {
			return "", "", "", err
		}
	}

	return forge, org, repoName, nil
}

func getGitHubOrgRepo(forge string, url string) (org string, repoName string, err error) {
	httpsPrefix := fmt.Sprintf("https://%s/", forge)
	sshPrefix := fmt.Sprintf("git@%s:", forge)

	if !strings.HasPrefix(url, httpsPrefix) && !strings.HasPrefix(url, sshPrefix) {
		return "", "", fmt.Errorf("GitHub URL does not start with 'https://%s/' or 'git@%s:': %s", forge, forge, url)
	}

	var suffix string
	if strings.HasPrefix(url, httpsPrefix) {
		suffix = url[len(httpsPrefix):]
	} else {
		suffix = url[len(sshPrefix):]
	}

	suffix = strings.TrimSuffix(suffix, ".git")
	parts := strings.Split(suffix, "/")

	if forge == gitHubForgeId {
		if len(parts) != 2 {
			return "", "", fmt.Errorf("unexpected URL format: %s", url)
		}
	} else if forge == gitlabForgeId {
		// TODO find limit
		if len(parts) > 3 {
			return "", "", fmt.Errorf("unexpected URL format: %s", url)
		}
	}

	org = parts[0]
	repoName = strings.Join(parts[1:], "/")

	return org, repoName, nil
}

func ignoreCommitAndParents(commit *object.Commit, commitMap map[plumbing.Hash]*CommitData, state *gitkit.RepoState) error {
	queue := []*object.Commit{commit}

	for {
		if len(queue) == 0 {
			break
		}

		current := queue[0]
		queue = queue[1:]

		c, found := commitMap[current.Hash]
		if found && c.AfterOrAncestorOfAfter {
			continue
		}

		for _, parentHash := range current.ParentHashes {
			parent, found := state.CommitMap[parentHash]
			if !found {
				return fmt.Errorf("failed to get parent commit %s", parentHash)
			}

			queue = append(queue, parent)
		}

		commitMap[current.Hash] = &CommitData{
			AfterOrAncestorOfAfter: true,
		}
	}

	return nil
}

func inferSignatureType(signature string) (SignatureType, error) {
	if strings.HasPrefix(signature, "-----BEGIN SSH SIGNATURE-----") {
		return SignatureTypeSSH, nil
	} else if strings.HasPrefix(signature, "-----BEGIN PGP SIGNATURE-----") {
		return SignatureTypePGP, nil
	} else if signature == "" {
		return SignatureTypeNone, nil
	} else {
		return SignatureTypeUnknown, fmt.Errorf("unknown signature type: '%s'", signature)
	}
}
