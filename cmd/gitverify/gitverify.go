package main

import (
	"bufio"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/supply-chain-tools/gitverify/gitverify"
	"github.com/supply-chain-tools/go-sandbox/githash"
	"github.com/supply-chain-tools/go-sandbox/gitkit"
	"github.com/supply-chain-tools/go-sandbox/hashset"
)

const usage = `Usage:
    gitverify [COMMAND] [OPTIONS]

COMMANDS
        verify
                Verify the state of a Git repository. This is also the default if no command is specified.
        after-candidates
                Generate a list of all commits that is not pointed to by other commits. The list can be
                used as the 'after' config.
        exempt-tags
                Generate a list of all the tags in the repository to be used for the 'exemptTags' config.

VERIFY OPTIONS
        --config-file
                Config file to use.
        --repository-uri
                URI to the repository in the config file.
        --commit
                Verify the commit.
        --tag
                Verify the tag and that it points to --commit.
        --branch
                Verify branch and ensure that --commit is on the branch.
        --verify-at-tip
                Verify that --commit is at the tip of --branch.
        --verify-at-head
                verify that HEAD points to the --commit. On by default.

AFTER-CANDIDATES OPTIONS
        --config-file
                Config file to use.
        --repository-uri
                URI to the repository in the config file.
        --sha512
                Output SHA-512 hashes in addition to SHA-1.

EXEMPT-TAGS OPTIONS
        --sha512
                Output SHA-512 hashes in addition to SHA-1.

GLOBAL OPTIONS
        --help, -h
                Show help
        --debug
                Output debug information.

Verify current repo
    $ gitverify

Verify current repo, specify config file and uri
    $ gitverify --config-file gitverify.json --repository-uri git+https://github.com/supply-chain-tools/go-sandbox.git

Verify repo and make sure a given commit and tag is present, that the tag points to the commit, that the commit
is on branch 'main' and that the commit is a descendant of 'after'
    $ gitverify --commit 1f46f2053221c040ce5bcba0239bc09214a37658 --tag v0.0.1 --branch main`

var packRegex = regexp.MustCompile("^pack-[a-f0-9]{40}\\.(pack|idx|rev|mtimes)$")

const (
	exitCodeOK              = 0
	exitCodeWorktreeChanges = 1
	exitCodeErr             = 3
)

func main() {
	flag.Usage = func() {
		fmt.Println(usage)
	}

	err := checkForUnsupportedEnvironmentVariables()
	if err != nil {
		printError(err)
		os.Exit(exitCodeErr)
	}

	command := "verify"
	if len(os.Args) > 1 {
		c := os.Args[1]
		if !strings.HasPrefix(c, "-") {
			command = os.Args[1]
		}
	}

	switch command {
	case "verify":
		opts, err := parseVerifyOptions(os.Args)
		if err != nil {
			printError(fmt.Errorf("failed to parse input: %w", err))
			os.Exit(exitCodeErr)
		}

		outputString, exitCode, err := verify(opts)
		if err != nil {
			printError(fmt.Errorf("verification failed: %w", err))
			os.Exit(exitCodeErr)
		}

		fmt.Print(outputString)
		os.Exit(exitCode)
	case "after-candidates":
		opts, err := parseGenerateOptions(os.Args[2:])
		if err != nil {
			printError(fmt.Errorf("failed to parse input: %w", err))
			os.Exit(exitCodeErr)
		}

		err = afterCandidates(opts)
		if err != nil {
			printError(fmt.Errorf("after-candidates failed: %w", err))
			os.Exit(exitCodeErr)
		}
	case "exempt-tags":
		opts, err := parseGenerateOptions(os.Args[2:])
		if err != nil {
			printError(fmt.Errorf("failed to parse input: %w", err))
			os.Exit(exitCodeErr)
		}

		result, err := exemptTags(opts)
		if err != nil {
			printError(fmt.Errorf("failed to get exempt tags: %w", err))
			os.Exit(exitCodeErr)
		}
		fmt.Println(result)
	default:
		printError(fmt.Errorf("unknown command: %s", command))
		os.Exit(exitCodeErr)
	}
}

func checkForUnsupportedEnvironmentVariables() error {
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

var allowedErrorCharactersRegex = regexp.MustCompile("^[a-zA-Z0-9 -_.@+/:\"']$")

func printError(err error) {
	rawErrorMessage := err.Error()
	sb := strings.Builder{}

	for i := range rawErrorMessage {
		if allowedErrorCharactersRegex.Match([]byte{rawErrorMessage[i]}) {
			sb.WriteByte(rawErrorMessage[i])
		} else {
			sb.WriteString(fmt.Sprintf("\\%.3d", rawErrorMessage[i]))
		}
	}

	print(sb.String(), "\n")
}

type VerifyOptions struct {
	repoDir         string
	validateOptions *gitverify.ValidateOptions
	configFilePath  string
	repoUri         string
	localState      bool
}

func parseVerifyOptions(osArgs []string) (*VerifyOptions, error) {
	flags := flag.NewFlagSet("all", flag.ExitOnError)
	var help, h, debugMode, verifyAtHEAD, verifyAtTip, localState, version, insecurePartialVerification bool
	var configFilePath, repoUri, commit, tag, branch string
	flags.BoolVar(&help, "help", false, "")
	flags.BoolVar(&h, "h", false, "")
	flags.BoolVar(&version, "version", false, "")
	flags.BoolVar(&debugMode, "debug", false, "")

	flags.StringVar(&configFilePath, "config-file", "", "")
	flags.StringVar(&repoUri, "repository-uri", "", "")
	flags.BoolVar(&localState, "local-state", true, "")

	flags.StringVar(&commit, "commit", "", "")
	flags.StringVar(&tag, "tag", "", "")
	flags.StringVar(&branch, "branch", "", "")
	flags.BoolVar(&verifyAtHEAD, "verify-at-head", true, "")
	flags.BoolVar(&verifyAtTip, "verify-at-tip", false, "")
	flags.BoolVar(&insecurePartialVerification, "insecure-partial-verification", false, "")

	args := osArgs[1:]
	if len(osArgs) > 2 && !strings.HasPrefix(osArgs[1], "-") {
		args = osArgs[2:]
	}

	err := flags.Parse(args)
	if err != nil || help || h {
		fmt.Println(usage)
		os.Exit(exitCodeOK)
	}

	if len(flags.Args()) > 0 {
		return nil, fmt.Errorf("no arguments expected, got: %s", strings.Join(flags.Args(), ","))
	}

	if version {
		err := printVersion()
		if err != nil {
			print("failed to print version: ", err.Error(), "\n")
			os.Exit(exitCodeErr)
		}
		os.Exit(exitCodeOK)
	}

	configureLogger(debugMode)

	repoDir, err := getRepoDir()
	if err != nil {
		return nil, err
	}

	if branch != "" && (commit == "" && tag == "") {
		return nil, fmt.Errorf("when using --branch, --commit or --tag must be specified")
	}

	if verifyAtTip && (commit == "" && tag == "") {
		return nil, fmt.Errorf("when using --verify-at-tip, --commit or --tag must be specified")
	}

	validateOptions := &gitverify.ValidateOptions{
		Commit:                      commit,
		Tag:                         tag,
		Branch:                      branch,
		VerifyAtHEAD:                verifyAtHEAD,
		VerifyAtTip:                 verifyAtTip,
		InsecurePartialVerification: insecurePartialVerification,
	}

	if configFilePath != "" || repoUri != "" {
		if configFilePath == "" {
			return nil, fmt.Errorf("--config-file must be used with --repository-uri")
		}

		if repoUri == "" {
			return nil, fmt.Errorf("--repository-uri must be used with --config-file\n")
		}

		// TODO consider supporting local state
		localState = false
	}

	if commit != "" {
		// TODO consider supporting local state
		localState = false
	}

	return &VerifyOptions{
		repoDir:         repoDir,
		validateOptions: validateOptions,
		configFilePath:  configFilePath,
		repoUri:         repoUri,
		localState:      localState,
	}, nil
}

type GenerateOptions struct {
	repoDir        string
	useSHA512      bool
	configFilePath string
	repoUri        string
}

func parseGenerateOptions(args []string) (*GenerateOptions, error) {
	var debugMode, useSHA512, help, h bool
	var configFilePath, repoUri string
	flags := flag.NewFlagSet("generate", flag.ExitOnError)
	flags.BoolVar(&debugMode, "debug", false, "")
	flags.BoolVar(&useSHA512, "sha512", false, "")
	flags.StringVar(&configFilePath, "config-file", "", "")
	flags.StringVar(&repoUri, "repository-uri", "", "")

	flags.BoolVar(&help, "help", false, "")
	flags.BoolVar(&h, "h", false, "")

	err := flags.Parse(args)
	if err != nil || help || h {
		fmt.Println(usage)
		os.Exit(exitCodeOK)
	}

	if len(flags.Args()) > 0 {
		return nil, fmt.Errorf("no arguments expected, got: %s", strings.Join(flags.Args(), ","))
	}

	configureLogger(debugMode)

	repoDir, err := getRepoDir()
	if err != nil {
		return nil, err
	}

	return &GenerateOptions{
		repoDir:        repoDir,
		useSHA512:      useSHA512,
		configFilePath: configFilePath,
		repoUri:        repoUri,
	}, nil
}

func getRepoDir() (string, error) {
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

func configureLogger(debugMode bool) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if debugMode {
		opts.Level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}

func verify(opts *VerifyOptions) (string, int, error) {
	repoDir := opts.repoDir
	validateOptions := opts.validateOptions
	configFilePath := opts.configFilePath
	repoUri := opts.repoUri
	localState := opts.localState

	repo, err := gitkit.OpenRepoInLocalPath(repoDir)
	if err != nil {
		return "", exitCodeErr, err
	}

	err = checkForUnsupportedGitPaths(repoDir)
	if err != nil {
		return "", exitCodeErr, err
	}

	state := gitkit.LoadRepoState(repo)
	sha1Hash := githash.NewGitHashFromRepoState(state, sha1.New())
	sha512Hash := githash.NewGitHashFromRepoState(state, sha512.New())

	var localStatePath string

	var repoConfig *gitverify.RepoConfig
	repoConfig, repoUri, err = loadRepoConfig(repo, configFilePath, repoUri)
	if err != nil {
		return "", exitCodeErr, err
	}

	err = gitverify.Verify(repo, state, repoConfig, sha1Hash, sha512Hash, validateOptions)
	if err != nil {
		return "", exitCodeErr, err
	}

	if localState {
		if configFilePath == "" {
			forge, org, repoName, err := gitverify.InferForgeOrgAndRepo(repo)
			if err != nil {
				return "", exitCodeErr, err
			}

			localStatePath, err = gitverify.GetLocalStatePath(forge, org, repoName)
			if err != nil {
				return "", exitCodeErr, err
			}
		}

		err = gitverify.VerifyLocalState(repo, state, repoConfig, repoUri, localStatePath, sha1Hash, sha512Hash)
		if err != nil {
			return "", exitCodeErr, fmt.Errorf("failed to verify local state: %w", err)
		}

		err = gitverify.SaveLocalState(repo, state, repoConfig, repoUri, localStatePath, sha1Hash, sha512Hash)
		if err != nil {
			return "", exitCodeErr, fmt.Errorf("failed to save local state: %w", err)
		}
	}

	return buildFinalOutput(repo, repoDir, repoUri, state, repoConfig, validateOptions)
}

func buildFinalOutput(repo *git.Repository, repoDir string, repoUri string, state *gitkit.RepoState, repoConfig *gitverify.RepoConfig, validateOpts *gitverify.ValidateOptions) (string, int, error) {
	sb := strings.Builder{}

	head, err := repo.Head()
	if err != nil {
		return "", exitCodeErr, err
	}

	referenceName := head.Name().String()

	commit := head.Hash()
	matchingTagRefs := make([]string, 0)
	matchingRemoteRefs := make([]string, 0)
	matchingHeadRefs := make([]string, 0)
	if commit.String() != "" {
		refs, err := repo.References()
		if err != nil {
			return "", exitCodeErr, err
		}

		err = refs.ForEach(func(reference *plumbing.Reference) error {
			if reference.Type() == plumbing.HashReference {
				protected, branchName := gitverify.IsProtected(reference, repoConfig)

				suffix := ""
				if protected {
					suffix = " [protected]"
				}
				if branchName != "" {
					if reference.Hash() == commit {
						if strings.HasPrefix(reference.Name().String(), "refs/heads/") {
							if referenceName != reference.Name().String() {
								matchingHeadRefs = append(matchingHeadRefs, branchName+suffix)
							}
						} else {
							referenceName := strings.TrimPrefix(reference.Name().String(), "refs/remotes/")
							matchingRemoteRefs = append(matchingRemoteRefs, referenceName+suffix)
						}
					}
				}

				if strings.HasPrefix(reference.Name().String(), "refs/tags/") {
					t, isAnnotatedTag := state.TagMap[reference.Hash()]

					tagName := strings.TrimPrefix(reference.Name().String(), "refs/tags/")

					if isAnnotatedTag {
						if t.Target.String() == commit.String() {
							matchingTagRefs = append(matchingTagRefs, "tag: "+tagName)
						}
					} else {
						if reference.Hash() == commit {
							matchingTagRefs = append(matchingTagRefs, "tag: "+tagName)
						}
					}
				}
			}

			return nil
		})
		if err != nil {
			return "", exitCodeErr, err
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", exitCodeErr, err
	}
	rel, err := filepath.Rel(dir, repoDir)
	if err != nil {
		return "", exitCodeErr, err
	}

	sb.WriteString(fmt.Sprintf("repository root: %s\n", rel))
	sb.WriteString(fmt.Sprintf("URI: %s\n", repoUri))

	worktree, err := repo.Worktree()
	if err != nil {
		return "", exitCodeErr, err
	}

	status, err := worktree.Status()
	if err != nil {
		return "", exitCodeErr, err
	}

	if validateOpts != nil && (validateOpts.Commit != "" || validateOpts.Tag != "" || validateOpts.Branch != "") {
		prefix := ""
		if validateOpts.InsecurePartialVerification {
			prefix = "[insecure] "
		}

		if validateOpts.VerifyAtHEAD {
			m := prefix + "partial verification OK"
			if status.IsClean() {
				sb.WriteString(fmt.Sprintf("working tree clean\n%s\n", m))
				return sb.String(), exitCodeOK, nil
			}

			sb.WriteString(fmt.Sprintf("there are worktree changes\notherwise %s\n", m))
			return sb.String(), exitCodeWorktreeChanges, nil
		}

		m := prefix + "partial detached verification OK"
		sb.WriteString(fmt.Sprintf("%s\n", m))
		return sb.String(), exitCodeWorktreeChanges, nil
	}

	matchingRefs := append(matchingHeadRefs, matchingTagRefs...)
	matchingRefs = append(matchingRefs, matchingRemoteRefs...)
	if strings.HasPrefix(referenceName, "refs/heads/") {
		ref := plumbing.NewReferenceFromStrings(referenceName, head.Hash().String())
		matchingRefs = append(matchingRefs, "commit: "+commit.String())

		protected, branchName := gitverify.IsProtected(ref, repoConfig)
		suffix := ""
		if protected {
			suffix = " [protected]"
		}

		if len(matchingRefs) > 0 {
			suffix += fmt.Sprintf(" (%s)", strings.Join(matchingRefs, ", "))
		}

		sb.WriteString(fmt.Sprintf("on branch %s%s\n", branchName, suffix))
	} else if head.Type() == plumbing.HashReference {
		suffix := ""
		if len(matchingRefs) > 0 {
			suffix += fmt.Sprintf(" (%s)", strings.Join(matchingRefs, ","))
		}

		sb.WriteString(fmt.Sprintf("on commit %s%s\n", head.Hash().String(), suffix))
	} else {
		return "", exitCodeErr, fmt.Errorf("HEAD broken")
	}

	m := "OK"
	if status.IsClean() {
		sb.WriteString(fmt.Sprintf("working tree clean\n%s\n", m))
		return sb.String(), exitCodeOK, nil
	}

	sb.WriteString(fmt.Sprintf("there are worktree changes\notherwise %s\n", m))
	return sb.String(), exitCodeWorktreeChanges, nil
}

func checkForUnsupportedGitPaths(repoDir string) error {
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

			if !gitverify.Hex2Regex.MatchString(name) {
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
				if !gitverify.HexSHA1Regex.MatchString(objectHash) {
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
			if !gitverify.HexSHA1Regex.MatchString(line[1:]) {
				return fmt.Errorf("unexpected peeled hash on line %d in %s", lineNumber, packedRefsPath)
			}

			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("unexpected line %d in %s", lineNumber, packedRefsPath)
		}

		hash := parts[0]

		if !gitverify.HexSHA1Regex.MatchString(hash) {
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

func loadRepoConfig(repo *git.Repository, configFilePath string, inputRepoUri string) (config *gitverify.RepoConfig, repoUri string, err error) {
	repoUri = inputRepoUri
	if configFilePath == "" {
		forge, org, repoName, err := gitverify.InferForgeOrgAndRepo(repo)
		if err != nil {
			return nil, "", err
		}

		configFilePath, err = gitverify.GetConfigPath(forge, org)
		if err != nil {
			return nil, "", err
		}

		repoUri = "git+https://" + forge + "/" + org + "/" + repoName + ".git"
	}

	parsedConfig, err := gitverify.LoadConfig(configFilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config: %w", err)
	}

	repoConfig, err := gitverify.LoadRepoConfig(parsedConfig, repoUri)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse config %s: %w", configFilePath, err)
	}

	return repoConfig, repoUri, nil
}

func afterCandidates(opts *GenerateOptions) error {
	repoDir := opts.repoDir
	useSHA512 := opts.useSHA512

	repo, err := gitkit.OpenRepoInLocalPath(repoDir)
	if err != nil {
		return fmt.Errorf("failed to open repo: %w", err)
	}

	err = checkForUnsupportedGitPaths(repoDir)
	if err != nil {
		return err
	}

	repoConfig, _, err := loadRepoConfig(repo, opts.configFilePath, opts.repoUri)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	candidates, err := gitverify.AfterCandidates(repo, repoConfig, useSHA512)
	if err != nil {
		return fmt.Errorf("failed to find after candidates: %w", err)
	}

	refs, err := repo.References()
	if err != nil {
		return fmt.Errorf("failed to list refs: %w", err)
	}

	refMap := make(map[plumbing.Hash][]string)
	err = refs.ForEach(func(reference *plumbing.Reference) error {
		ref, found := refMap[reference.Hash()]

		if found {
			refMap[reference.Hash()] = append(ref, reference.Name().String())
		} else {
			refMap[reference.Hash()] = []string{reference.Name().String()}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to process refs: %w", err)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Branch != nil && candidates[j].Branch != nil {
			return *candidates[i].Branch < *candidates[j].Branch
		}

		if candidates[i].Branch != nil {
			return true
		}

		if candidates[j].Branch != nil {
			return false
		}

		return *candidates[i].SHA1 < *candidates[j].SHA1
	})

	for i, candidate := range candidates {
		refs, found := refMap[plumbing.NewHash(*candidate.SHA1)]
		if found {
			fmt.Printf("%s %s\n", *candidate.SHA1, strings.Join(refs, ","))

			if candidates[i].Branch == nil {
				allBranches := hashset.New[string]()
				var branchName string
				for _, ref := range refs {
					branchName, found = gitverify.BranchName(ref)
					if found {
						allBranches.Add(branchName)
					}
				}

				if allBranches.Size() == 1 {
					candidates[i].Branch = &allBranches.Values()[0]
				}
			}
		}
	}

	data, err := json.Marshal(candidates)
	if err != nil {
		return fmt.Errorf("failed to marshal candidates: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func exemptTags(opts *GenerateOptions) (string, error) {
	repoDir := opts.repoDir
	useSHA512 := opts.useSHA512

	repo, err := gitkit.OpenRepoInLocalPath(repoDir)
	if err != nil {
		return "", err
	}

	err = checkForUnsupportedGitPaths(repoDir)
	if err != nil {
		return "", err
	}

	state := gitkit.LoadRepoState(repo)
	sha1Hash := githash.NewGitHashFromRepoState(state, sha1.New())
	sha512Hash := githash.NewGitHashFromRepoState(state, sha512.New())
	exemptTags, err := gitverify.ComputeExemptTags(repo, state, sha1Hash, sha512Hash, useSHA512)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(exemptTags)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func printVersion() error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Errorf("no version information")
	}

	for _, kv := range info.Settings {
		if strings.HasPrefix(kv.Key, "vcs") {
			fmt.Printf("%s: %s\n", kv.Key, kv.Value)
		}
	}

	return nil
}
