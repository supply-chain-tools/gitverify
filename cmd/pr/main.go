package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/supply-chain-tools/gitverify/pr"
)

const usage = `Usage:
    pr [COMMAND] [OPTIONS] <PR number>

COMMANDS
        tag
                Create a tag for a PR (creates the tag 'pr/<PR number>'.
        merge
                Merge a PR tag (merge tag 'pr/<PR number>' into the current branch).

OPTIONS
        --message
                Message to include in tag/commit.

Create a tag for PR 1
    $ pr tag 1

Merge PR tag 1 with a message
    $ pr merge --message "fixes" 1
`

func main() {
	optionsAndArgs, err := parseOptionsAndArgs()
	if err != nil {
		print(err.Error(), "\n")
		os.Exit(1)
	}

	switch optionsAndArgs.command {
	case tag:
		err = pr.Tag(optionsAndArgs.prNumber, optionsAndArgs.message)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}
	case merge:
		err = pr.Merge(optionsAndArgs.prNumber, optionsAndArgs.message)
		if err != nil {
			print(err.Error(), "\n")
			os.Exit(1)
		}
	default:
		print("Unknown command: ", optionsAndArgs.command, "\n")
		os.Exit(1)
	}
}

type optionsAndArgs struct {
	command  Command
	prNumber int
	message  string
}

type Command string

const (
	tag   Command = "tag"
	merge Command = "merge"
)

func parseOptionsAndArgs() (*optionsAndArgs, error) {
	flag.Usage = func() {
		fmt.Print(usage)
	}

	flags := flag.NewFlagSet("all", flag.ExitOnError)
	var help, h, debugMode bool
	var messageInput string
	var prNumber int

	flags.BoolVar(&help, "help", false, "")
	flags.BoolVar(&h, "h", false, "")
	flags.BoolVar(&debugMode, "debug", false, "")
	flags.StringVar(&messageInput, "message", "", "")

	if len(os.Args) < 2 {
		print(usage)
		os.Exit(1)
	}
	commandString := os.Args[1]

	err := flags.Parse(os.Args[2:])
	if err != nil || help || h {
		print(usage)
		os.Exit(0)
	}

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if debugMode {
		opts.Level = slog.LevelDebug
	}

	command, err := getCommand(commandString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %s", commandString)
	}

	if len(flags.Args()) != 1 {
		return nil, fmt.Errorf("pr number is required")
	}

	prNumber, err = strconv.Atoi(flags.Args()[0])
	if err != nil {
		return nil, fmt.Errorf("pr number is invalid: %s", flags.Args()[0])
	}

	if prNumber <= 0 {
		return nil, fmt.Errorf("pr number must be positive: %d", prNumber)
	}

	// FIXME verify message characters

	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	return &optionsAndArgs{
		command:  command,
		message:  messageInput,
		prNumber: prNumber,
	}, nil
}

func getCommand(commandString string) (Command, error) {
	switch commandString {
	case string(tag):
		return tag, nil
	case string(merge):
		return merge, nil
	default:
		return "", fmt.Errorf("unknown command: %s", commandString)
	}
}
