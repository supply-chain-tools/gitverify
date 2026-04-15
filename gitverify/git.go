package gitverify

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-git/go-git/v5"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/supply-chain-tools/go-sandbox/gitkit"
)

type SignatureType string

const (
	SignatureTypeGPG     SignatureType = "gpg"
	SignatureTypeSSH     SignatureType = "ssh"
	SignatureTypeNone    SignatureType = "none"
	SignatureTypeSMime   SignatureType = "smime"
	SignatureTypeUnknown SignatureType = "unknown"
	namespaceSSH         string        = "git"
)

type CommitData struct {
	SignatureType     SignatureType
	Ignore            bool
	SignatureVerified bool
	MergeTag          *object.Tag
}

func InferForgeOrgAndRepo(repo *git.Repository) (forge string, org string, repoName string) {
	remote, err := repo.Remote("origin")
	if err != nil {
		log.Fatal(err)
	}
	urls := remote.Config().URLs
	if len(urls) != 1 {
		log.Fatal("Expected exactly one remote url")
	}

	org, repoName, err = getGitHubOrgRepo(urls[0])
	if err != nil {
		log.Fatal(err)
	}

	return gitHubForgeId, org, repoName
}

func getGitHubOrgRepo(url string) (org string, repoName string, err error) {
	const httpsPrefix = "https://github.com/"
	const sshPrefix = "git@github.com:"

	if !strings.HasPrefix(url, httpsPrefix) && !strings.HasPrefix(url, sshPrefix) {
		return "", "", fmt.Errorf("GitHub URL does not start with 'https://github.com/' or 'git@github.com:': %s", url)
	}

	var suffix string
	if strings.HasPrefix(url, httpsPrefix) {
		suffix = url[len(httpsPrefix):]
	} else {
		suffix = url[len(sshPrefix):]
	}

	suffix = strings.TrimSuffix(suffix, ".git")
	parts := strings.Split(suffix, "/")

	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected URL format: %s", url)
	}

	org = parts[0]
	repoName = parts[1]

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
		if found && c.Ignore {
			continue
		}

		for _, parentHash := range current.ParentHashes {
			parent, found := state.CommitMap[parentHash]
			if !found {
				return fmt.Errorf("failed to get parent commit %s", parentHash)
			}

			queue = append(queue, parent)
		}

		signatureType, err := inferSignatureType(current.PGPSignature)
		if err != nil {
			return err
		}

		commitMap[current.Hash] = &CommitData{
			SignatureType: signatureType,
			Ignore:        true,
		}
	}

	return nil
}

func inferSignatureType(signature string) (SignatureType, error) {
	if strings.HasPrefix(signature, "-----BEGIN SSH SIGNATURE-----") {
		return SignatureTypeSSH, nil
	} else if strings.HasPrefix(signature, "-----BEGIN PGP SIGNATURE-----") {
		return SignatureTypeGPG, nil
	} else if signature == "" {
		return SignatureTypeNone, nil
	} else {
		return SignatureTypeUnknown, fmt.Errorf("unknown signature type: '%s'", signature)
	}
}
