package gitverify

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/supply-chain-tools/go-sandbox/githash"
	"github.com/supply-chain-tools/go-sandbox/gitkit"
	"github.com/supply-chain-tools/go-sandbox/hashset"
)

type ValidateOptions struct {
	Commit       string
	Tag          string
	Branch       string
	VerifyOnHEAD bool
	VerifyOnTip  bool
}

var Hex2Regex = regexp.MustCompile("^[a-f0-9]{2}$")
var HexSHA1Regex = regexp.MustCompile("^[a-f0-9]{40}$")
var HexSHA512Regex = regexp.MustCompile("^[a-f0-9]{128}$")

func Verify(repo *git.Repository, state *gitkit.RepoState, repoConfig *RepoConfig, gitHashSHA1 githash.GitHash, gitHashSHA512 githash.GitHash, opts *ValidateOptions) error {
	commitMetadata, err := computeCommitMetadata(state, repoConfig, gitHashSHA1, gitHashSHA512)
	if err != nil {
		return err
	}

	remoteSet, err := getRemoteSet(repo)
	if err != nil {
		return err
	}

	err = validateRefs(repo, state, repoConfig, remoteSet)
	if err != nil {
		return err
	}

	if opts != nil && opts.Commit != "" {
		if !HexSHA1Regex.MatchString(opts.Commit) {
			return fmt.Errorf("target commit must be a 40 character hex, not '%s'", opts.Commit)
		}

		err = validateOpts(opts, repo, state, commitMetadata, repoConfig, gitHashSHA1, gitHashSHA512)
		if err != nil {
			return err
		}
	} else {
		if repoConfig.lockdown {
			for _, commit := range state.CommitMap {
				err := validateCommit(commit, commitMetadata, repoConfig)
				if err != nil {
					return err
				}
			}
		}

		err = validateTags(repo, state, repoConfig, gitHashSHA1, gitHashSHA512)
		if err != nil {
			return err
		}

		err = validateProtectedBranches(repo, state, commitMetadata, repoConfig, gitHashSHA512)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateRefs(repo *git.Repository, state *gitkit.RepoState, repoConfig *RepoConfig, remoteSet hashset.Set[string]) error {
	refsIter, err := repo.References()
	if err != nil {
		return err
	}

	refSet := hashset.New[string]()
	err = refsIter.ForEach(func(reference *plumbing.Reference) error {
		refSet.Add(reference.Name().String())
		return nil
	})
	if err != nil {
		return err
	}

	refsIter, err = repo.References()
	if err != nil {
		return err
	}

	err = refsIter.ForEach(func(reference *plumbing.Reference) error {
		referenceName := reference.Name().String()

		if referenceName == "HEAD" {
			switch reference.Type() {
			case plumbing.SymbolicReference:
				targetName := reference.Target().String()
				if !refSet.Contains(targetName) {
					return fmt.Errorf("symbolic reference '%s' pointed to by HEAD not found", targetName)
				}

				if targetName == "HEAD" {
					return fmt.Errorf("HEAD is pointing to itself")
				}
			case plumbing.HashReference:
				_, found := state.CommitMap[reference.Hash()]
				if !found {
					return fmt.Errorf("did not find commit %s pointed to by HEAD", reference.Hash().String())
				}
			default:
				return fmt.Errorf("unsupported reference type %s for HEAD", reference.Type().String())
			}

			return nil
		}

		if strings.HasPrefix(referenceName, "refs/remotes/") {
			parts := strings.Split(referenceName, "/")
			remote := parts[2]
			if !remoteSet.Contains(remote) {
				return fmt.Errorf("reference %s does not match any remote", referenceName)
			}

			name := strings.Join(parts[3:], "")
			if name == "" {
				return fmt.Errorf("reference %s is too short", referenceName)
			}

			if name == "HEAD" {
				if reference.Type() != plumbing.SymbolicReference {
					return fmt.Errorf("reference %s is expected to be a symbolic", referenceName)
				}

				targetName := reference.Target().String()
				expectedPrefix := fmt.Sprintf("refs/remotes/%s/", remote)
				if !strings.HasPrefix(targetName, expectedPrefix) {
					return fmt.Errorf("reference %s is expected to start with %s", referenceName, expectedPrefix)
				}

				if !refSet.Contains(targetName) {
					return fmt.Errorf("the target of reference %s is missing", targetName)
				}

				targetBranchName := strings.TrimPrefix(targetName, expectedPrefix)
				if !repoConfig.protectedBranches.Contains(targetBranchName) {
					return fmt.Errorf("reference %s must point to a protected branch", referenceName)
				}
			} else {
				if reference.Type() != plumbing.HashReference {
					return fmt.Errorf("reference %s is expected to be a hash", referenceName)
				}

				_, found := state.CommitMap[reference.Hash()]
				if !found {
					return fmt.Errorf("did not find commit %s for reference %s", reference.Hash().String(), referenceName)
				}
			}
		} else if strings.HasPrefix(referenceName, "refs/heads/") {
			if reference.Type() != plumbing.HashReference {
				return fmt.Errorf("reference %s is expected to be a hash", referenceName)
			}

			_, found := state.CommitMap[reference.Hash()]
			if !found {
				return fmt.Errorf("did not find commit %s for reference %s", reference.Hash().String(), referenceName)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func getRemoteSet(repo *git.Repository) (hashset.Set[string], error) {
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

func validateCommit(commit *object.Commit, commitMetadata map[plumbing.Hash]*CommitData, repoConfig *RepoConfig) error {
	metadata, found := commitMetadata[commit.Hash]
	if !found {
		return fmt.Errorf("commit not processed: %s", commit.Hash)
	}

	if metadata.Ignore || metadata.SignatureVerified {
		return nil
	}

	email := commit.Committer.Email

	if repoConfig.trustedForge != nil {
		if repoConfig.trustedForge.email == email {
			err := validateGPGCommit(commit, repoConfig.trustedForge.gpgPublicKey)
			if err != nil {
				return err
			}

			_, found := repoConfig.maintainerOrContributorEmails[commit.Author.Email]
			if !found {
				_, found := repoConfig.maintainerOrContributorForgeEmails[commit.Author.Email]
				if !found {
					return fmt.Errorf("author email '%s' not found for forge commit: %s", commit.Author.Email, commit.Hash.String())
				}
			}

			return nil
		}
	}

	id, found := repoConfig.maintainerOrContributorEmails[email]
	if !found {
		return fmt.Errorf("no maintainer with email '%s' for commit %s", email, commit.Hash)
	}

	switch metadata.SignatureType {
	case SignatureTypeSSH:
		content, err := buildContent(commit)
		if err != nil {
			return err
		}
		err = validateSSH(content, commit.PGPSignature, id, repoConfig)
		if err != nil {
			return fmt.Errorf("failed to validate commit %s: %w", commit.Hash.String(), err)
		}
	case SignatureTypeGPG:
		err := validateIdentityGPGCommit(commit, id, repoConfig)
		if err != nil {
			return err
		}
	case SignatureTypeNone:
		return fmt.Errorf("unsigned commit: %s", commit.Hash.String())
	default:
		return fmt.Errorf("unknown signature type for commit: %s", commit.Hash.String())
	}

	if commit.MergeTag != "" {
		mergeTag, err := extractMergeTag(commit)
		if err != nil {
			return err
		}

		signatureType, err := inferSignatureType(mergeTag.PGPSignature)
		if err != nil {
			return err
		}

		id, found := repoConfig.maintainerEmails[mergeTag.Tagger.Email]
		if !found {
			return fmt.Errorf("no maintainer with email '%s' for mergetag in commit %s", mergeTag.Tagger.Email, commit.Hash.String())
		}

		switch signatureType {
		case SignatureTypeSSH:
			content, err := tagContent(mergeTag)
			if err != nil {
				return err
			}
			err = validateSSH(content, mergeTag.PGPSignature, id, repoConfig)
			if err != nil {
				return fmt.Errorf("failed to validate mergetag in commit %s: %w", commit.Hash.String(), err)
			}
		case SignatureTypeGPG:
			err := validateIdentityGPGTag(mergeTag, id, repoConfig)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("missing signature for mergetag in commit %s", commit.Hash.String())
		}

		if len(commit.ParentHashes) != 2 {
			return fmt.Errorf("expected two parents when using mergetag in commit %s", commit.Hash.String())
		}

		if commit.ParentHashes[1] != mergeTag.Target {
			return fmt.Errorf("commit parent does not match mergetag in commit %s", commit.Hash.String())
		}

		metadata.MergeTag = mergeTag
	}

	metadata.SignatureVerified = true

	return nil
}

func extractMergeTag(commit *object.Commit) (*object.Tag, error) {
	if commit.MergeTag == "" {
		return nil, fmt.Errorf("commit does not contain mergetag: %s", commit.Hash.String())
	}

	memoryObject := plumbing.MemoryObject{}
	memoryObject.SetType(plumbing.TagObject)
	_, err := memoryObject.Write([]byte(commit.MergeTag))
	if err != nil {
		return nil, err
	}

	mergeTag := object.Tag{}
	err = mergeTag.Decode(&memoryObject)
	if err != nil {
		return nil, err
	}

	return &mergeTag, nil
}

func validateOpts(opts *ValidateOptions, repo *git.Repository, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData, config *RepoConfig, gitHashSHA1 githash.GitHash, gitHashSHA512 githash.GitHash) error {
	head, err := repo.Head()
	if err != nil {
		return err
	}

	headHash := head.Hash()

	c, found := state.CommitMap[plumbing.NewHash(opts.Commit)]
	if !found {
		return fmt.Errorf("target commit '%s' not found", opts.Commit)
	}

	err = validateCommitsRecursively(c, state, commitMetadata, config)
	if err != nil {
		return err
	}

	targetHash := c.Hash

	if opts.VerifyOnHEAD {
		if targetHash != headHash {
			return fmt.Errorf("HEAD does not point to the target commit %s", opts.Commit)
		}
	}

	var tagHash *plumbing.Hash = nil
	if opts.Tag != "" {
		tags, err := repo.Tags()
		if err != nil {
			return err
		}

		tagFound := false
		err = tags.ForEach(func(tag *plumbing.Reference) error {
			tagName := strings.TrimPrefix(tag.Name().String(), "refs/tags/")

			if tagName == opts.Tag {
				err := validateTag(tag, state, config, gitHashSHA1, gitHashSHA512)
				if err != nil {
					return err
				}

				t, found := state.TagMap[tag.Hash()]
				if found {
					// annotated tag
					tagHash = &t.Target
				} else {
					// lightweight tag
					t := tag.Hash()
					tagHash = &t
				}

				tagFound = true
			}
			return nil
		})
		if err != nil {
			return err
		}

		if !tagFound {
			return fmt.Errorf("target tag '%s' not found", opts.Tag)
		}
	}

	if tagHash != nil {
		if targetHash != *tagHash {
			return fmt.Errorf("target tag '%s' does not point to target commit %s ", opts.Tag, targetHash)
		}
	}

	if opts.Branch != "" {
		remotes, err := repo.References()
		if err != nil {
			return err
		}

		branchFound := false
		err = remotes.ForEach(func(reference *plumbing.Reference) error {
			isProtected, branchName := isProtected(reference, config)

			if branchName == opts.Branch {
				branchFound = true

				c, found := state.CommitMap[reference.Hash()]
				if !found {
					return fmt.Errorf("commit '%s' not found", reference.Hash().String())
				}

				err := validateCommitsRecursively(c, state, commitMetadata, config)
				if err != nil {
					return err
				}

				if isProtected {
					err := validateProtectedBranch(reference, branchName, state, commitMetadata, config, gitHashSHA512)
					if err != nil {
						return fmt.Errorf("failed to validate protected branch '%s' rules: %w", reference.Name(), err)
					}
				}

				if opts.VerifyOnTip {
					if targetHash != c.Hash {
						return fmt.Errorf("target commit %s does not point to the tip of branch '%s'", targetHash.String(), reference.Name())
					}
				} else {
					err = validateOnBranch(targetHash, branchName, c, state, commitMetadata, config)
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		if !branchFound {
			return fmt.Errorf("target branch '%s' not found", opts.Branch)
		}
	}

	return nil
}

func validateOnBranch(targetHash plumbing.Hash, branchName string, c *object.Commit, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData, config *RepoConfig) error {
	current := c

	for {
		if current.Hash == targetHash {
			break
		}

		if len(current.ParentHashes) == 0 {
			return fmt.Errorf("target commit %s is not on target branch '%s'", targetHash, branchName)
		}

		parentHash := current.ParentHashes[0]
		parent, found := state.CommitMap[parentHash]
		if !found {
			return fmt.Errorf("target parent hash not found: %s", parentHash)
		}

		err := validateCommit(parent, commitMetadata, config)
		if err != nil {
			return err
		}

		current = parent
	}

	return nil
}

func validateCommitsRecursively(c *object.Commit, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData, config *RepoConfig) error {
	err := validateCommit(c, commitMetadata, config)
	if err != nil {
		return err
	}

	visited := hashset.New[plumbing.Hash]()
	visited.Add(c.Hash)
	queue := []*object.Commit{c}

	for {
		if len(queue) == 0 {
			break
		}

		current := queue[0]
		queue = queue[1:]

		for _, parentHash := range current.ParentHashes {
			if !visited.Contains(parentHash) {
				parent, found := state.CommitMap[parentHash]
				if !found {
					return fmt.Errorf("target parent hash not found: %s", parentHash)
				}

				if !commitMetadata[parent.Hash].Ignore {
					err := validateCommit(parent, commitMetadata, config)
					if err != nil {
						return err
					}

					queue = append(queue, parent)
					visited.Add(parentHash)
				}
			}

			if !config.lockdown {
				// Only check first parent
				break
			}
		}
	}

	return nil
}

func verifyConnectedToSpecificAfter(commit *object.Commit, after plumbing.Hash, state *gitkit.RepoState, allowCommitsBeforeAfter bool) error {
	if commit.Hash == after {
		return nil
	}

	afterCommit, found := state.CommitMap[after]
	if !found {
		return fmt.Errorf("target after hash not found: %s", after.String())
	}

	// see if commit is a descendant of after
	connected, err := isLeftDescendant(commit, afterCommit, state)
	if err != nil {
		return err
	}

	if connected {
		return nil
	}

	if !allowCommitsBeforeAfter {
		return fmt.Errorf("commit %s is not a descendant of after %s", commit.Hash.String(), after.String())
	}

	// see if after is a descendant of commit
	connected, err = isLeftDescendant(afterCommit, commit, state)
	if err != nil {
		return err
	}

	if !connected {
		return fmt.Errorf("commit %s is not connected to after %s", commit.Hash.String(), after.String())
	}

	return nil
}

func isLeftDescendant(a *object.Commit, b *object.Commit, state *gitkit.RepoState) (bool, error) {
	current := a

	for {
		if current.Hash == b.Hash {
			return true, nil
		}

		if len(current.ParentHashes) == 0 {
			return false, nil
		}

		parentHash := current.ParentHashes[0]

		parent, found := state.CommitMap[parentHash]
		if !found {
			return false, fmt.Errorf("target parent hash not found: %s", parentHash)
		}

		current = parent
	}
}

func validateProtectedBranches(repo *git.Repository, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData, config *RepoConfig, gitHashSHA512 githash.GitHash) error {
	remotes, err := repo.References()
	if err != nil {
		return err
	}

	err = remotes.ForEach(func(reference *plumbing.Reference) error {
		isProtected, branchName := isProtected(reference, config)

		if isProtected {
			err := validateProtectedBranch(reference, branchName, state, commitMetadata, config, gitHashSHA512)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func validateProtectedBranch(reference *plumbing.Reference, branchName string, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData, config *RepoConfig, gitHashSHA512 githash.GitHash) error {
	targetAfter, found := config.branchToSHA1[branchName]
	if !found {
		return fmt.Errorf("protected branch '%s' without matching after branch", branchName)
	}

	current, found := state.CommitMap[reference.Hash()]
	if !found {
		return fmt.Errorf("did not find commit %s", reference.Hash().String())
	}

	for {
		err := validateCommit(current, commitMetadata, config)
		if err != nil {
			return err
		}

		if current.Hash == targetAfter {
			break
		}

		if config.requireMergeCommits {
			if len(current.ParentHashes) != 2 {
				return fmt.Errorf("requireMergeCommits is set, but commit %s on protected branch has %d parents", current.Hash.String(), len(current.ParentHashes))
			}
		}

		if config.requireCountersigning {
			metadata := commitMetadata[current.Hash]

			if metadata.MergeTag == nil {
				return fmt.Errorf("requireCountersigning is set, but no mergetag in commit %s", current.Hash.String())
			}

			targetCommit, found := state.CommitMap[metadata.MergeTag.Target]
			if !found {
				return fmt.Errorf("mergetag target commit %s not found", metadata.MergeTag.Target.String())
			}

			if current.TreeHash != targetCommit.TreeHash {
				return fmt.Errorf("requireCountersigning is set, but commit tree and mergetag tree does not match for commit %s", current.Hash.String())
			}

			if metadata.MergeTag.Tagger.Email == current.Committer.Email {
				return fmt.Errorf("committer and tagger cannot be the same when requireCountersigning is set for commit %s", current.Hash.String())
			}

			err = verifyConnected(current.ParentHashes[1], current.ParentHashes[0], state)
			if err != nil {
				return err
			}

			if config.requireSHA512 {
				messageLines := strings.Split(metadata.MergeTag.Message, "\n")
				prefix := "Gitverify-object-sha512: "

				verified := false
				for i := len(messageLines) - 1; i >= 0; i-- {
					if strings.HasPrefix(messageLines[i], prefix) {
						hash := strings.TrimPrefix(messageLines[i], prefix)

						if !HexSHA512Regex.MatchString(hash) {
							return fmt.Errorf("malformed Gitverify-object-sha512 in merge commit: %s", current.Hash.String())
						}

						objectSHA512, err := gitHashSHA512.CommitSum(targetCommit.Hash)
						if err != nil {
							return err
						}

						if hex.EncodeToString(objectSHA512) == hash {
							// Note that this verifies the commit the mergetag points to, including its tree, which by the
							// check above has to point to the same tree that is in the merge commit.
							verified = true
							break
						}

						return fmt.Errorf("wrong Gitverify-object-sha512 in merge commit %s", current.Hash.String())
					}
				}

				if !verified {
					return fmt.Errorf("missing Gitverify-object-sha512 in merge commit %s", current.Hash.String())
				}
			}
		}

		if len(current.ParentHashes) == 2 {
			_, found := config.maintainerEmails[current.Committer.Email]
			if !found {
				if config.trustedForge != nil && current.Committer.Email == config.trustedForge.email {
					_, found = config.maintainerEmails[current.Author.Email]
					if !found {
						_, found = config.maintainerForgeEmails[current.Author.Email]
					}
				}

				if !found {
					return fmt.Errorf("merge commit %s made by %s which is not a maintainer", current.Hash.String(), current.Committer.Email)
				}
			}
		}

		if len(current.ParentHashes) == 0 {
			return fmt.Errorf("protected branch %s is not a descendant of after", reference.Name().String())
		}

		current, found = state.CommitMap[current.ParentHashes[0]]
		if !found {
			return fmt.Errorf("did not find commit %s", reference.Hash().String())
		}
	}

	return nil
}

func verifyConnected(start plumbing.Hash, target plumbing.Hash, state *gitkit.RepoState) error {
	if start == target {
		return fmt.Errorf("start and target must be different, got %s", start.String())
	}

	queue := []plumbing.Hash{start}
	for len(queue) > 0 {
		currentHash := queue[0]
		queue = queue[1:]

		if currentHash == target {
			return nil
		}

		current, found := state.CommitMap[currentHash]
		if !found {
			return fmt.Errorf("did not find commit %s", currentHash.String())
		}

		for _, p := range current.ParentHashes {
			queue = append(queue, p)
		}
	}

	return fmt.Errorf("no path from %s to %s", start.String(), target.String())
}

func validateTags(repo *git.Repository, state *gitkit.RepoState, repoConfig *RepoConfig, gitHashSHA1 githash.GitHash, gitHashSHA512 githash.GitHash) error {
	tags, err := repo.Tags()
	if err != nil {
		return err
	}

	err = tags.ForEach(func(tag *plumbing.Reference) error {
		return validateTag(tag, state, repoConfig, gitHashSHA1, gitHashSHA512)
	})
	if err != nil {
		return err
	}

	return nil
}

func validateTag(tag *plumbing.Reference, state *gitkit.RepoState, repoConfig *RepoConfig, gitHashSHA1 githash.GitHash, gitHashSHA512 githash.GitHash) error {
	isExempted := false

	t, isAnnotatedTag := state.TagMap[tag.Hash()]
	if isAnnotatedTag {
		if t.Hash != tag.Hash() {
			return fmt.Errorf("inconsistent hash for tag %s", tag.Hash().String())
		}

		hash := t.Hash
		verifiedHash, err := gitHashSHA1.TagSum(hash)
		if err != nil {
			return err
		}

		if !bytes.Equal(verifiedHash, hash[:]) {
			return fmt.Errorf("failed to verify hash for annotated tag %s", tag.Hash().String())
		}
	} else {
		hash := tag.Hash()
		verifiedHash, err := gitHashSHA1.CommitSum(tag.Hash())
		if err != nil {
			return err
		}

		if !bytes.Equal(verifiedHash, hash[:]) {
			return fmt.Errorf("failed to verify hash for light weight tag %s", tag.Hash().String())
		}
	}

	tagHash, found := repoConfig.exemptedTags[tag.Name().String()]
	if found {
		if tagHash != tag.Hash().String() {
			return fmt.Errorf("wrong hash.sha1 for exempted tag '%s', got %s, expected %s", tag.Name().String(), tag.Hash().String(), tagHash)
		}
		isExempted = true
	}

	tagHashSHA512, found := repoConfig.exemptedTagsSHA512[tag.Name().String()]
	if found {
		var sha512Hash []byte
		var err error
		if isAnnotatedTag {
			sha512Hash, err = gitHashSHA512.TagSum(t.Hash)
			if err != nil {
				return err
			}
		} else {
			sha512Hash, err = gitHashSHA512.CommitSum(tag.Hash())
			if err != nil {
				return err
			}
		}

		h := hex.EncodeToString(sha512Hash)
		if tagHashSHA512 != h {
			return fmt.Errorf("wrong SHA-512 for exempted tag '%s', got %s, expected %s", tag.Name().String(), h, tagHashSHA512)
		}
		isExempted = true
	}

	entry := strings.TrimPrefix(tag.Name().String(), "refs/tags/")
	if isAnnotatedTag {
		if entry != t.Name {
			return fmt.Errorf("tag ref '%s' does not match name '%s'", entry, t.Name)
		}

		if !isExempted {
			signatureType, err := inferSignatureType(t.PGPSignature)
			if err != nil {
				return err
			}

			id, found := repoConfig.maintainerEmails[t.Tagger.Email]
			if !found {
				return fmt.Errorf("no maintainer with email '%s' for tag %s", t.Tagger.Email, t.Name)
			}

			switch signatureType {
			case SignatureTypeSSH:
				content, err := tagContent(t)
				if err != nil {
					return err
				}
				err = validateSSH(content, t.PGPSignature, id, repoConfig)
				if err != nil {
					return fmt.Errorf("failed to validate tag %s: %w", t.Name, err)
				}
			case SignatureTypeGPG:
				err := validateIdentityGPGTag(t, id, repoConfig)
				if err != nil {
					return err
				}
			case SignatureTypeNone:
				if repoConfig.requireSignedTags {
					return fmt.Errorf("unsigned annotated tag: %s", t.Name)
				}
			default:
				return fmt.Errorf("unknown signature type for tag: %s", t.Name)
			}
		}
	} else {
		if !isExempted {
			if repoConfig.requireSignedTags {
				return fmt.Errorf("tag '%s' is lightweight, but signing is required", tag.Name())
			}
		}
	}

	return nil
}

func tagContent(tag *object.Tag) (string, error) {
	if tag.TargetType != plumbing.CommitObject {
		return "", fmt.Errorf("only commit tags are supported, got '%s' for tag '%s'", tag.Type(), tag.Name)
	}

	memoryObject := &plumbing.MemoryObject{}
	err := tag.EncodeWithoutSignature(memoryObject)
	if err != nil {
		return "", err
	}

	reader, err := memoryObject.Reader()
	if err != nil {
		return "", err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func computeCommitMetadata(state *gitkit.RepoState, repoConfig *RepoConfig, gitHashSHA1 githash.GitHash, gitHashSHA512 githash.GitHash) (map[plumbing.Hash]*CommitData, error) {
	commitMap := make(map[plumbing.Hash]*CommitData)

	foundAfterSHA1 := hashset.New[plumbing.Hash]()
	foundAfterSHA512 := hashset.New[[64]byte]()

	for hash, commit := range state.CommitMap {
		if len(commit.ParentHashes) > 2 {
			return nil, fmt.Errorf("up to two parents are allowed, commit '%s' has %d", hash.String(), len(commit.ParentHashes))
		}

		verifiedSHA1, err := gitHashSHA1.CommitSum(hash)
		if err != nil {
			return nil, err
		}

		if !bytes.Equal(verifiedSHA1, hash[:]) {
			return nil, fmt.Errorf("failed to verify hash %s", hash)
		}

		var verifiedSHA512 [64]byte
		var sha512WasVerified = false
		if repoConfig.afterSHA512.Size() > 0 {
			v, err := gitHashSHA512.CommitSum(hash)
			if err != nil {
				return nil, err
			}

			if len(v) != 64 {
				return nil, fmt.Errorf("expected hash to be 64, got %d", len(v))
			}

			copy(verifiedSHA512[:], v[:])
			sha512WasVerified = true
		}

		matchedAfterSHA512 := false
		if sha512WasVerified {
			if repoConfig.afterSHA512.Contains(verifiedSHA512) {
				matchedAfterSHA512 = true
				foundAfterSHA512.Add(verifiedSHA512)
			}
		}

		matchedAfterSHA1 := false
		if repoConfig.afterSHA1.Size() > 0 {
			if repoConfig.afterSHA1.Contains(hash) {
				matchedAfterSHA1 = true
				foundAfterSHA1.Add(hash)
			}
		}

		matchedAfter := false

		_, found := repoConfig.afterSHA1ToSHA512[hash]
		if found {
			// Both SHA-1 and SHA-512 specified, check that they are the same
			if matchedAfterSHA1 != matchedAfterSHA512 {
				return nil, fmt.Errorf("matched after SHA-1 or SHA-512 but not both")
			}

			matchedAfter = matchedAfterSHA1
		} else {
			// Otherwise it's enough that one matched
			matchedAfter = matchedAfterSHA1 || matchedAfterSHA512
		}

		if matchedAfter {
			if !repoConfig.afterSHA1.Contains(hash) {
				// This was matched using SHA-512, add it to SHA-1 as well
				repoConfig.afterSHA1.Add(hash)

				// Use branches from SHA-512
				branch := repoConfig.sha512ToBranch[verifiedSHA512]
				repoConfig.sha1ToBranch[hash] = branch
				repoConfig.branchToSHA1[branch] = hash
			}
		}

		_, found = commitMap[hash]
		if found {
			continue
		}

		if matchedAfter {
			err := ignoreCommitAndParents(commit, commitMap, state)
			if err != nil {
				return nil, err
			}
		} else {
			signatureType, err := inferSignatureType(commit.PGPSignature)
			if err != nil {
				return nil, err
			}

			commitMap[hash] = &CommitData{
				SignatureType: signatureType,
			}
		}
	}

	afterSHA1Diff := repoConfig.afterSHA1.Difference(foundAfterSHA1)
	if afterSHA1Diff.Size() > 0 {
		missingHashes := make([]string, 0)
		for _, k := range afterSHA1Diff.Values() {
			missingHashes = append(missingHashes, k.String())
		}
		return nil, fmt.Errorf("after SHA-1 commit(s) not found in repo: %s", strings.Join(missingHashes, ","))
	}

	afterSHA512Diff := repoConfig.afterSHA512.Difference(foundAfterSHA512)
	if afterSHA512Diff.Size() > 0 {
		missingHashes := make([]string, 0)
		for _, k := range afterSHA512Diff.Values() {
			missingHashes = append(missingHashes, hex.EncodeToString(k[:]))
		}
		return nil, fmt.Errorf("after SHA-512 commit(s) not found in repo: %s", strings.Join(missingHashes, ","))
	}

	return commitMap, nil
}

func buildContent(commit *object.Commit) (string, error) {
	memoryObject := &plumbing.MemoryObject{}
	err := commit.EncodeWithoutSignature(memoryObject)
	if err != nil {
		return "", err
	}

	reader, err := memoryObject.Reader()
	if err != nil {
		return "", err
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func isProtected(reference *plumbing.Reference, config *RepoConfig) (bool, string) {
	isProtected := false
	var branchName string
	referenceName := reference.Name().String()
	if strings.HasPrefix(referenceName, "refs/remotes/") {
		parts := strings.Split(referenceName, "/")
		branchName = strings.Join(parts[3:], "/")
		if config.protectedBranches.Contains(branchName) {
			isProtected = true
		}
	} else if strings.HasPrefix(referenceName, "refs/heads/") {
		branchName = strings.TrimPrefix(referenceName, "refs/heads/")
		if config.protectedBranches.Contains(branchName) {
			isProtected = true
		}
	}

	return isProtected, branchName
}

func BranchName(ref string) (string, bool) {
	found := false
	var branchName string

	remotesPrefix := "refs/remotes/"
	headsPrefix := "refs/heads/"
	if strings.HasPrefix(ref, remotesPrefix) {
		suffix := strings.TrimPrefix(ref, remotesPrefix)
		parts := strings.Split(suffix, "/")
		branchName = strings.Join(parts[1:], "/")
		found = true
	} else if strings.HasPrefix(ref, headsPrefix) {
		branchName = strings.TrimPrefix(ref, headsPrefix)
		found = true
	}

	return branchName, found
}
