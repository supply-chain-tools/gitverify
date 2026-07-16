package main

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/supply-chain-tools/gitverify/gitverify"
	"github.com/supply-chain-tools/go-sandbox/gitkit"
)

type commandType string

const (
	commandFetch commandType = "fetch"
	commandPull  commandType = "pull"
	commandPush  commandType = "push"
	commandClone commandType = "clone"

	commandInit     commandType = "init"
	commandCheckout commandType = "checkout"
	commandSwitch   commandType = "switch"

	commandStatus commandType = "status"
	commandDiff   commandType = "diff"
	commandGrep   commandType = "grep"
	commandShow   commandType = "show"
	commandLog    commandType = "log"

	commandAdd    commandType = "add"
	commandMv     commandType = "mv"
	commandCommit commandType = "commit"
	commandMerge  commandType = "merge"
	commandRebase commandType = "rebase"
	commandReset  commandType = "reset"
	commandBranch commandType = "branch"
	commandTag    commandType = "tag"
)

var plainNameRegex = regexp.MustCompile("^[a-z]+$")
var repoNameRegex = regexp.MustCompile("^[a-zA-Z]+\\.git$")

func main() {
	optionsAndArgs, err := parse()
	if err != nil {
		print(err.Error(), "\n")
		os.Exit(1)
	}

	err = gitverify.CheckForUnsupportedEnvironmentVariables()
	if err != nil {
		print(err.Error(), "\n")
		os.Exit(1)
	}

	if optionsAndArgs.command == commandClone {
		err = runClone(optionsAndArgs)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}

		err := os.Chdir(*optionsAndArgs.repoDir)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}

		err = runGitverify()
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}

		os.Exit(0)
	}

	repo, err := openRepo()
	if err != nil {
		print(err.Error(), "\n")
		os.Exit(1)
	}

	err = verifyRemote(optionsAndArgs, repo)
	if err != nil {
		print(err.Error(), "\n")
		os.Exit(1)
	}

	switch optionsAndArgs.command {
	case commandFetch:
		err = runFetch(optionsAndArgs)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}

		err = runGitverify()
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}
	case commandPull:
		branch, err := verifyBranch(optionsAndArgs, repo)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}

		err = runFetch(optionsAndArgs)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}

		err = runGitverify()
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}

		err = runLocalMerge(optionsAndArgs, branch)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}
	case commandPush:
		_, err := verifyBranch(optionsAndArgs, repo)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}

		err = runGitverify()
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}

		err = runPush(optionsAndArgs)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}
	case commandInit:
		fallthrough
	case commandCheckout:
		fallthrough
	case commandSwitch:
		fallthrough
	case commandStatus:
		fallthrough
	case commandDiff:
		fallthrough
	case commandGrep:
		fallthrough
	case commandShow:
		fallthrough
	case commandLog:
		fallthrough
	case commandAdd:
		fallthrough
	case commandMv:
		fallthrough
	case commandCommit:
		fallthrough
	case commandMerge:
		fallthrough
	case commandRebase:
		fallthrough
	case commandReset:
		fallthrough
	case commandBranch:
		fallthrough
	case commandTag:
		err := gitPassthrough(optionsAndArgs)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}
	default:
		print("unknown command\n")
		os.Exit(1)
	}
}

func verifyBranch(optionsAndArgs *OptionsAndArgs, repo *git.Repository) (string, error) {
	currentRef, err := currentBranch(repo)
	if err != nil {
		return "", err
	}

	if optionsAndArgs.ref != nil {
		if *optionsAndArgs.ref != currentRef {
			return "", fmt.Errorf("actual branch %s differ from remote branch %s", currentRef, *optionsAndArgs.ref)
		}
	}

	return currentRef, nil
}

func verifyRemote(optionsAndArgs *OptionsAndArgs, repo *git.Repository) error {
	remoteSet, err := gitverify.GetRemoteSet(repo)
	if err != nil {
		return err
	}

	if optionsAndArgs.remote != nil {
		if !remoteSet.Contains(*optionsAndArgs.remote) {
			return fmt.Errorf("remote %s does not exist", *optionsAndArgs.remote)
		}
	}

	return nil
}

func openRepo() (*git.Repository, error) {
	repoDir, err := gitverify.GetRepoDir()
	if err != nil {
		return nil, err
	}

	repo, err := gitkit.OpenRepoInLocalPath(repoDir)
	if err != nil {
		return nil, err
	}

	err = gitverify.CheckForUnsupportedGitPaths(repoDir)
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func currentBranch(repo *git.Repository) (string, error) {
	ref, err := repo.Head()
	if err != nil {
		return "", err
	}

	refName := ref.Name().String()
	refPrefix := "refs/heads/"
	if !strings.HasPrefix(refName, refPrefix) {
		return "", fmt.Errorf("not on a branch in 'refs/heads'")
	}

	candidate := strings.TrimPrefix(refName, refPrefix)
	// TODO proper check
	if !plainNameRegex.MatchString(candidate) {
		return "", fmt.Errorf("illegal characters in branch name")
	}

	return candidate, nil
}

func runPush(optionsAndArgs *OptionsAndArgs) error {
	command := []string{"git", "push", *optionsAndArgs.remote, *optionsAndArgs.ref}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func gitPassthrough(optionsAndArgs *OptionsAndArgs) error {
	command := []string{"git", string(optionsAndArgs.command)}
	command = append(command, optionsAndArgs.args...)

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func runFetch(optionsAndArgs *OptionsAndArgs) error {
	command := []string{"git", "fetch"}

	if optionsAndArgs.remote != nil {
		command = append(command, *optionsAndArgs.remote)
	}

	if optionsAndArgs.ref != nil {
		command = append(command, *optionsAndArgs.ref)
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func runLocalMerge(optionsAndArgs *OptionsAndArgs, branch string) error {
	remote := *optionsAndArgs.remote
	ref := remote + "/" + branch

	command := []string{"git", "merge", ref, "--ff-only"}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func runClone(optionsAndArgs *OptionsAndArgs) error {
	command := []string{"git", "clone", *optionsAndArgs.url}
	fmt.Printf("command: %s\n", command)

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func runGitverify() error {
	command := []string{"gitverify"}

	cmd := exec.Command(command[0])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

type OptionsAndArgs struct {
	command commandType
	args    []string
	remote  *string
	ref     *string
	url     *string
	repoDir *string
}

func parse() (*OptionsAndArgs, error) {
	if len(os.Args) < 2 {
		return nil, fmt.Errorf("missing command")
	}

	args := os.Args[1:]

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return nil, fmt.Errorf("unexpected prefix '-' for %s", arg)
		} else if strings.HasPrefix(arg, "--") {
			return nil, fmt.Errorf("unexpected prefix '--' for %s", arg)
		}
	}

	var remote *string = nil
	var ref *string = nil
	var err error
	var cloneUrl *string = nil
	var repoDir *string = nil

	var command commandType
	switch args[0] {
	case string(commandFetch):
		command = commandFetch
		remote, ref, err = getRemoteAndRef(args)
		if err != nil {
			return nil, err
		}
	case string(commandPull):
		command = commandPull
		remote, ref, err = getMandatoryRemoteAndRef(args)
		if err != nil {
			return nil, err
		}
	case string(commandPush):
		command = commandPush
		remote, ref, err = getMandatoryRemoteAndRef(args)
		if err != nil {
			return nil, err
		}
	case string(commandClone):
		command = commandClone
		cloneUrl, repoDir, err = getCloneUrlAndRepoDir(args)
		if err != nil {
			return nil, err
		}
	case string(commandInit):
		command = commandInit
	case string(commandCheckout):
		command = commandCheckout
	case string(commandSwitch):
		command = commandSwitch
	case string(commandStatus):
		command = commandStatus
	case string(commandDiff):
		command = commandDiff
	case string(commandGrep):
		command = commandGrep
	case string(commandShow):
		command = commandShow
	case string(commandLog):
		command = commandLog
	case string(commandAdd):
		command = commandAdd
	case string(commandMv):
		command = commandMv
	case string(commandCommit):
		command = commandCommit
	case string(commandMerge):
		command = commandMerge
	case string(commandRebase):
		command = commandRebase
	case string(commandReset):
		command = commandReset
	case string(commandBranch):
		command = commandBranch
	case string(commandTag):
		command = commandTag
	default:
		return nil, fmt.Errorf("unknown command %s", command)
	}

	return &OptionsAndArgs{
		command: command,
		args:    args[1:],
		remote:  remote,
		ref:     ref,
		url:     cloneUrl,
		repoDir: repoDir,
	}, nil
}

func getMandatoryRemoteAndRef(args []string) (*string, *string, error) {
	remote, ref, err := getRemoteAndRef(args)
	if err != nil {
		return nil, nil, err
	}

	if remote == nil {
		return nil, nil, fmt.Errorf("remote must be specified")
	}

	if ref == nil {
		return nil, nil, fmt.Errorf("branch must be specified")
	}

	return remote, ref, nil
}

func getRemoteAndRef(args []string) (*string, *string, error) {
	var remote *string = nil
	var ref *string = nil

	if len(args) >= 2 {
		candidate := args[1]
		if !plainNameRegex.MatchString(candidate) {
			return nil, nil, fmt.Errorf("invalid remote %s", candidate)
		}
		remote = &args[1]
	}

	if len(args) >= 3 {
		candidate := args[2]
		// TODO proper check
		if !plainNameRegex.MatchString(candidate) {
			return nil, nil, fmt.Errorf("invalid branch %s", candidate)
		}

		ref = &args[2]
	}

	if len(args) >= 4 {
		return nil, nil, fmt.Errorf("too many arguments")
	}

	return remote, ref, nil
}

func getCloneUrlAndRepoDir(args []string) (*string, *string, error) {
	if len(args) < 2 {
		return nil, nil, fmt.Errorf("repository URL must be specified")
	}

	if len(args) > 2 {
		return nil, nil, fmt.Errorf("too many arguments")
	}

	cloneUr := args[1]
	repoDir := ""
	if strings.HasPrefix(cloneUr, "https://") {
		u, err := url.Parse(cloneUr)
		if err != nil {
			return nil, nil, err
		}

		c := filepath.Base(u.Path)
		if !repoNameRegex.MatchString(c) {
			return nil, nil, fmt.Errorf("invalid repository URL")
		}
		repoDir = strings.TrimSuffix(c, ".git")
	} else if strings.HasPrefix(cloneUr, "git@") {
		rest := strings.TrimPrefix(cloneUr, "git@")
		parts := strings.Split(rest, ":")
		if len(parts) != 2 {
			fmt.Printf("here 1\n")
			return nil, nil, fmt.Errorf("invalid repository URL")
		}

		host := parts[0]
		// FIXME
		u, err := url.Parse("https://" + host)
		if err != nil {
			return nil, nil, err
		}

		if u.Host != host {
			return nil, nil, fmt.Errorf("invalid repository URL")
		}

		path := parts[1]
		if !fs.ValidPath(path) {
			return nil, nil, fmt.Errorf("invalid repository URL")
		}

		c := filepath.Base(path)
		if !repoNameRegex.MatchString(c) {
			fmt.Printf("here 4\n")
			return nil, nil, fmt.Errorf("invalid repository URL")
		}
		repoDir = strings.TrimSuffix(c, ".git")
	} else {
		fmt.Printf("here 6\n")
		return nil, nil, fmt.Errorf("invalid repository URL prefix")
	}

	return &cloneUr, &repoDir, nil
}
