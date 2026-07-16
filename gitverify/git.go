package gitverify

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/supply-chain-tools/go-sandbox/hashset"

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

var packRegex = regexp.MustCompile("^pack-[a-f0-9]{40}\\.(pack|idx|rev|mtimes)$")

type CommitData struct {
	AfterOrAncestorOfAfter bool
	ConnectedToAfter       bool
	OnProtectedBranch      bool
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

func GetRepoDir() (string, error) {
	basePath, err := os.Getwd()
	if err != nil {
		return "", err
	}

	repoDir, found, err := gitkit.GetRootPathOfLocalGitRepo(basePath)
	if err != nil {
		return "", err
	}

	if !found {
		return "", fmt.Errorf("no repository found in %s", basePath)
	}

	return repoDir, nil
}

func GetRemoteSet(repo *git.Repository) (hashset.Set[string], error) {
	remoteSet := hashset.New[string]()
	r, err := repo.Remotes()
	if err != nil {
		return nil, err
	}

	for _, remote := range r {
		candidate := remote.Config().Name
		if strings.Contains(candidate, "/") {
			return nil, fmt.Errorf("remote '%s' contains '/'", candidate)
		}
		remoteSet.Add(candidate)
	}

	return remoteSet, nil
}

func CheckForUnsupportedEnvironmentVariables() error {
	// https://git-scm.com/docs/git#_environment_variables
	unsupportedEnvironmentVariables := hashset.New[string](
		"GIT_DIR",
		"GIT_CEILING_DIRECTORIES",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_NAMESPACE",
		"GIT_COMMON_DIR",
	)

	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		key := pair[0]
		if unsupportedEnvironmentVariables.Contains(key) {
			return fmt.Errorf("environment variable %s is not supported", key)
		}
	}

	return nil
}

func CheckForUnsupportedGitPaths(repoDir string) error {
	// https://git-scm.com/docs/git-replace
	replacePath := filepath.Join(repoDir, ".git", "refs", "replace")
	if _, err := os.Stat(replacePath); err == nil {
		return fmt.Errorf("git replace is not supported")
	}

	// https://git-scm.com/docs/git-replace#Documentation/git-replace.txt---graftcommitparent
	graftPath := filepath.Join(repoDir, ".git", "info", "grafts")
	if _, err := os.Stat(graftPath); err == nil {
		return fmt.Errorf("git graft is not supported")
	}

	// https://git-scm.com/docs/git-clone#Documentation/git-clone.txt---shared
	alternatesPath := filepath.Join(repoDir, ".git", "objects", "info", "alternates")
	if _, err := os.Stat(alternatesPath); err == nil {
		return fmt.Errorf("git alternates is not supported")
	}

	// https://git-scm.com/docs/gitnamespaces
	namespacesPath := filepath.Join(repoDir, ".git", "refs", "namespaces")
	if _, err := os.Stat(namespacesPath); err == nil {
		return fmt.Errorf("git namespaces is not supported")
	}

	// https://git-scm.com/docs/shallow
	shallowPath := filepath.Join(repoDir, ".git", "shallow")
	if _, err := os.Stat(shallowPath); err == nil {
		return fmt.Errorf("git shallow is not supported")
	}

	// https://git-scm.com/docs/reftable
	reftablePath := filepath.Join(repoDir, ".git", "reftable")
	if _, err := os.Stat(reftablePath); err == nil {
		return fmt.Errorf("git reftable is not supported")
	}

	err := checkForRefSymlinks(repoDir)
	if err != nil {
		return err
	}

	err = checkForUnsupportedPackFiles(repoDir)
	if err != nil {
		return err
	}

	err = checkObjectsDir(repoDir)
	if err != nil {
		return err
	}

	err = checkPackedRefs(repoDir)
	if err != nil {
		return err
	}

	return nil
}

func checkForRefSymlinks(repoDir string) error {
	refDir := filepath.Join(repoDir, ".git", "refs")
	err := filepath.Walk(refDir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !(info.Mode().IsDir() || info.Mode().IsRegular()) {
				return fmt.Errorf("only directories and regular files are supported in refs dir: %s", path)
			}

			return nil
		})
	if err != nil {
		return err
	}

	return nil
}

func checkForUnsupportedPackFiles(repoDir string) error {
	packDir := filepath.Join(repoDir, ".git", "objects", "pack")

	entries, err := os.ReadDir(packDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported non-regular file in %s", packDir)
		}

		name := entry.Name()
		if name == "multi-pack-index" {
			// https://git-scm.com/docs/multi-pack-index
			return fmt.Errorf("git multi-pack-index is not supported")
		}

		if strings.HasSuffix(name, ".bitmap") {
			// https://git-scm.com/docs/bitmap-format
			return fmt.Errorf("git bitmap is not supported")
		}

		if !packRegex.Match([]byte(name)) {
			return fmt.Errorf("unsupported name of file %s in %s", entry.Name(), packDir)
		}
	}

	return nil
}

func checkObjectsDir(repoDir string) error {
	objectsDir := filepath.Join(repoDir, ".git", "objects")

	entries, err := os.ReadDir(objectsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()

		if len(name) == 2 {
			if !entry.Type().IsDir() {
				return fmt.Errorf("%s in %s expected to be a directory", name, objectsDir)
			}

			if !Hex2Regex.MatchString(name) {
				return fmt.Errorf("%s in %s expected to be hex", name, objectsDir)
			}

			objects, err := os.ReadDir(filepath.Join(objectsDir, name))
			if err != nil {
				return err
			}

			for _, object := range objects {
				if !object.Type().IsRegular() {
					return fmt.Errorf("object %s in %s expected to be a regular file", object.Name(), filepath.Join(objectsDir, name))
				}

				objectHash := name + object.Name()
				if !HexSHA1Regex.MatchString(objectHash) {
					return fmt.Errorf("object %s in %s expected to be a SHA-1 hash", objectHash, objectsDir)
				}
			}
		}
	}

	return nil
}

func checkPackedRefs(repoDir string) error {
	// https://git-scm.com/docs/git-pack-refs
	packedRefsPath := filepath.Join(repoDir, ".git", "packed-refs")

	file, err := os.Open(packedRefsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	scanner := bufio.NewScanner(file)
	lineNumber := 0

	refsSet := hashset.New[string]()
	for scanner.Scan() {
		lineNumber++

		line := scanner.Text()
		if lineNumber == 1 {
			if strings.HasPrefix(line, "#") {
				continue
			} else {
				return fmt.Errorf("unexpected first line of %s", packedRefsPath)
			}
		}

		if strings.HasPrefix(line, "^") {
			if !HexSHA1Regex.MatchString(line[1:]) {
				return fmt.Errorf("unexpected peeled hash on line %d in %s", lineNumber, packedRefsPath)
			}

			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("unexpected line %d in %s", lineNumber, packedRefsPath)
		}

		hash := parts[0]

		if !HexSHA1Regex.MatchString(hash) {
			return fmt.Errorf("expected line %d in %s to start with a SHA-1 hash", lineNumber, packedRefsPath)
		}

		ref := parts[1]
		if !strings.HasPrefix(ref, "refs/") {
			return fmt.Errorf("expected line %d in %s to end with a ref", lineNumber, packedRefsPath)
		}

		if strings.HasPrefix(ref, "refs/replace/") {
			return fmt.Errorf("git replace is not supported in %s", packedRefsPath)
		}

		if strings.HasPrefix(ref, "refs/namespaces/") {
			return fmt.Errorf("git namespaces is not supported in %s", packedRefsPath)
		}

		if refsSet.Contains(ref) {
			return fmt.Errorf("duplicate ref at line %d in %s", lineNumber, packedRefsPath)
		}

		refsSet.Add(ref)
	}

	err = scanner.Err()
	closeErr := file.Close()
	if err != nil && closeErr != nil {
		return fmt.Errorf("%w %w", err, closeErr)
	} else if err != nil {
		return err
	} else if closeErr != nil {
		return closeErr
	}

	return nil
}
