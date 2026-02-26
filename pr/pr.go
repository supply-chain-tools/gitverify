package pr

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/supply-chain-tools/go-sandbox/githash"
	"github.com/supply-chain-tools/go-sandbox/gitkit"
	"github.com/supply-chain-tools/go-sandbox/gitverify"
)

func Tag(prNumber int, message string) error {
	tagName := fmt.Sprintf("pr/%d", prNumber)

	repo, err := openRepoFromCwd()
	if err != nil {
		return err
	}

	gh := githash.NewGitHash(repo, sha512.New())
	head, err := repo.Head()
	if err != nil {
		return err
	}

	objectSHA512, err := gh.CommitSum(head.Hash())
	if err != nil {
		return err
	}
	objectSHA512Hex := hex.EncodeToString(objectSHA512)

	sb := strings.Builder{}

	_, orgName, repoName := gitverify.InferForgeOrgAndRepo(repo)
	sb.WriteString(fmt.Sprintf("PR https://github.com/%s/%s/pull/%d\n\n", orgName, repoName, prNumber))

	if message != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", message))
	}

	sb.WriteString(fmt.Sprintf("Object-sha512: %s\n", objectSHA512Hex))

	m := sb.String()
	fmt.Print(m)

	err = createTag(m, tagName, head.Hash().String())
	if err != nil {
		return err
	}

	return nil
}

func Merge(prNumber int, message string) error {
	repo, err := openRepoFromCwd()
	if err != nil {
		return err
	}

	sb := strings.Builder{}

	_, orgName, repoName := gitverify.InferForgeOrgAndRepo(repo)
	sb.WriteString(fmt.Sprintf("Merged PR https://github.com/%s/%s/pull/%d\n\n", orgName, repoName, prNumber))

	if message != "" {
		sb.WriteString(fmt.Sprintf("%s\n\n", message))
	}

	m := sb.String()
	fmt.Print(m)

	tagName := fmt.Sprintf("pr/%d", prNumber)
	command := []string{"git", "merge", "-S", "-m", m, tagName}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return err
	}

	tag, err := repo.Tag(tagName)
	if err != nil {
		return err
	}

	tagObject, err := repo.TagObject(tag.Hash())
	if err != nil {
		return err
	}
	fmt.Printf("\nApprove PR: https://github.com/%s/%s/pull/%d/changes/%s\n", orgName, repoName, prNumber, tagObject.Target.String())

	return nil
}

func openRepoFromCwd() (*git.Repository, error) {
	basePath, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	repoDir, found, err := gitkit.GetRootPathOfLocalGitRepo(basePath)
	if err != nil {
		return nil, fmt.Errorf("unable infer git root from %s: %w", basePath, err)
	}

	if !found {
		return nil, fmt.Errorf("not in a git repo %s", basePath)
	}

	repo, err := gitkit.OpenRepoInLocalPath(repoDir)
	if err != nil {
		return nil, fmt.Errorf("unable to open repo %s: %w", repoDir, err)
	}

	return repo, nil
}

func createTag(message string, tag string, hash string) error {
	command := []string{"git", "tag", "-s", "-m", message, tag, hash}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}
