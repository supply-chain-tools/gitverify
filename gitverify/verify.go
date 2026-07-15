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
	Commit                      string
	Tag                         string
	Branch                      string
	VerifyAtHEAD                bool
	VerifyAtTip                 bool
	InsecurePartialVerification bool
}

var Hex2Regex = regexp.MustCompile("^[a-f0-9]{2}$")
var HexSHA1Regex = regexp.MustCompile("^[a-f0-9]{40}$")
var HexSHA512Regex = regexp.MustCompile("^[a-f0-9]{128}$")
var countersignTagRegex = regexp.MustCompile("^refs/tags/pr/[0-9]+$")

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

	if opts != nil && (opts.Commit != "" || opts.Tag != "" || opts.Branch != "") {
		if opts.Commit != "" && !HexSHA1Regex.MatchString(opts.Commit) {
			return fmt.Errorf("target commit must be a 40 character hex, not '%s'", opts.Commit)
		}

		err = validateOpts(opts, repo, state, commitMetadata, repoConfig, gitHashSHA1, gitHashSHA512)
		if err != nil {
			return err
		}
	} else {
		if repoConfig.verifyAllCommits {
			for _, commit := range state.CommitMap {
				err := validateCommit(commit, state, commitMetadata, gitHashSHA512, repoConfig)
				if err != nil {
					return err
				}

				err = verifyConnectedToAfter(commit, state, commitMetadata)
				if err != nil {
					return err
				}
			}
		}

		err = validateBranches(repo, state, commitMetadata, repoConfig, gitHashSHA512)
		if err != nil {
			return err
		}

		err = validateTags(repo, state, repoConfig, commitMetadata, gitHashSHA1, gitHashSHA512)
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

func validateCommit(commit *object.Commit, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData, gitHashSHA512 githash.GitHash, repoConfig *RepoConfig) error {
	metadata, found := commitMetadata[commit.Hash]
	if !found {
		return fmt.Errorf("commit not processed: %s", commit.Hash)
	}

	if metadata.AfterOrAncestorOfAfter || metadata.SignatureVerified {
		return nil
	}

	signatureType, err := inferSignatureType(commit.PGPSignature)
	if err != nil {
		return err
	}

	email := commit.Committer.Email

	if repoConfig.trustedForge != nil {
		if repoConfig.trustedForge.email == email {
			if commit.MergeTag != "" {
				return fmt.Errorf("forge cannot sign countersign commits")
			}

			switch signatureType {
			case SignatureTypePGP:
				key := repoConfig.trustedForge.commitPGPPublicKey
				if key == nil {
					return fmt.Errorf("wrong signature type PGP for forge commit %s", commit.Hash.String())
				}

				err := validatePGPCommit(commit, *key)
				if err != nil {
					return err
				}
			case SignatureTypeSSH:
				id := repoConfig.trustedForge.identity
				if id == nil {
					return fmt.Errorf("wrong signature type SSH for forge commit %s", commit.Hash.String())
				}

				content, err := buildContent(commit)
				if err != nil {
					return err
				}

				err = validateSSH(content, commit.PGPSignature, id.commitSSHPublicKeys, repoConfig)
				if err != nil {
					return fmt.Errorf("failed to validate commit %s: %w", commit.Hash.String(), err)
				}
			case SignatureTypeNone:
				return fmt.Errorf("unsigned forge commit: %s", commit.Hash.String())
			default:
				return fmt.Errorf("unknown signature type for forge commit: %s", commit.Hash.String())
			}

			_, found := repoConfig.maintainerOrContributorEmails[commit.Author.Email]
			if !found {
				_, found = repoConfig.maintainerOrContributorForgeEmails[commit.Author.Email]
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

	switch signatureType {
	case SignatureTypeSSH:
		content, err := buildContent(commit)
		if err != nil {
			return err
		}

		sshPublicKeys := id.commitSSHPublicKeys
		if commit.MergeTag != "" {
			sshPublicKeys = id.countersignCommitSSHPublicKeys
		}

		err = validateSSH(content, commit.PGPSignature, sshPublicKeys, repoConfig)
		if err != nil {
			return fmt.Errorf("failed to validate commit %s: %w", commit.Hash.String(), err)
		}
	case SignatureTypePGP:
		pgpPublicKeys := id.commitPGPPublicKeys
		if commit.MergeTag != "" {
			pgpPublicKeys = id.countersignCommitPGPPublicKeys
		}

		err := validateIdentityPGPCommit(commit, pgpPublicKeys, repoConfig)
		if err != nil {
			return err
		}
	case SignatureTypeNone:
		return fmt.Errorf("unsigned commit: %s", commit.Hash.String())
	default:
		return fmt.Errorf("unknown signature type for commit: %s", commit.Hash.String())
	}

	if commit.MergeTag != "" {
		if len(commit.ParentHashes) != 2 {
			return fmt.Errorf("expected exactly 2 parent commits for countersigned commit %s", commit.Hash.String())
		}

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
			err = validateSSH(content, mergeTag.PGPSignature, id.countersignTagSSHPublicKeys, repoConfig)
			if err != nil {
				return fmt.Errorf("failed to validate mergetag in commit %s: %w", commit.Hash.String(), err)
			}
		case SignatureTypePGP:
			err := validateIdentityPGPTag(mergeTag, id.countersignTagPGPPublicKeys, repoConfig)
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

		if repoConfig.requireDistinctCountersignTagIdentities {
			if mergeTag.Tagger.Email == email {
				return fmt.Errorf("requireDistinctCountersignTagIdentities is set but identity %s is reused in commit %s", email, commit.Hash.String())
			}
		}

		if repoConfig.requireDistinctCountersignCommitIdentities {
			if mergeTag.Tagger.Email == email {
				return fmt.Errorf("requireDistinctCountersignCommitIdentities is set but identity %s is reused in commit %s", email, commit.Hash.String())
			}
		}

		targetCommit, found := state.CommitMap[mergeTag.Target]
		if !found {
			return fmt.Errorf("mergetag target commit %s not found", metadata.MergeTag.Target.String())
		}

		if commit.TreeHash != targetCommit.TreeHash {
			return fmt.Errorf("countersigned trees do not match for countersigned commit %s", commit.Hash.String())
		}

		if repoConfig.requireSHA512 {
			messageLines := strings.Split(mergeTag.Message, "\n")

			err = verifySha512(commit.Hash, targetCommit.Hash, messageLines, gitHashSHA512)
			if err != nil {
				return err
			}
		}

		err = verifyConnected(commit.ParentHashes[1], commit.ParentHashes[0], state)
		if err != nil {
			return err
		}

		metadata.MergeTag = mergeTag
	}

	metadata.SignatureVerified = true

	return nil
}

func verifySha512(commitHash plumbing.Hash, targetCommitHash plumbing.Hash, messageLines []string, gitHashSHA512 githash.GitHash) error {
	prefix := "Gitverify-object-sha512: "

	verified := false
	for i := len(messageLines) - 1; i >= 0; i-- {
		if strings.HasPrefix(messageLines[i], prefix) {
			if verified {
				return fmt.Errorf("duplicate '%s' in commit %s", prefix, commitHash.String())
			}

			hash := strings.TrimPrefix(messageLines[i], prefix)

			if !HexSHA512Regex.MatchString(hash) {
				return fmt.Errorf("malformed Gitverify-object-sha512 in merge commit: %s", commitHash.String())
			}

			objectSHA512, err := gitHashSHA512.CommitSum(targetCommitHash)
			if err != nil {
				return err
			}

			if hex.EncodeToString(objectSHA512) != hash {
				return fmt.Errorf("wrong Gitverify-object-sha512 in merge commit %s", commitHash.String())
			}

			verified = true
		}
	}

	if !verified {
		return fmt.Errorf("missing Gitverify-object-sha512 in merge commit %s", commitHash.String())
	}

	return nil
}

func verifyVersionForTag(tagReference string, tagName string, commit *object.Commit, commitMetadata map[plumbing.Hash]*CommitData) error {
	if countersignTagRegex.MatchString(tagReference) {
		return nil
	}

	metadata, found := commitMetadata[commit.Hash]
	if !found {
		return fmt.Errorf("commit metadata for commit %s not found", commit.Hash.String())
	}

	commitLines := strings.Split(commit.Message, "\n")
	commitVersions := extractVersions(commitLines)

	if metadata.MergeTag != nil {
		tagLines := strings.Split(metadata.MergeTag.Message, "\n")
		tagVersions := extractVersions(tagLines)

		if len(tagVersions) != 1 {
			return fmt.Errorf("expected exactly one version for countersigned commit %s", commit.Hash.String())
		}

		if tagVersions[0] != tagName {
			return fmt.Errorf("countersigned commit version does not match tag name %s", tagName)
		}

		if len(commitVersions) != 0 {
			return fmt.Errorf("expected the version to be in the mergetag not the commit %s", commit.Hash.String())
		}
	} else {
		if len(commitVersions) != 1 {
			return fmt.Errorf("expected exactly one version in the commit %s", commit.Hash.String())
		}

		if commitVersions[0] != tagName {
			return fmt.Errorf("commit version does not match tag name %s", tagName)
		}
	}

	return nil
}

func extractVersions(messageLines []string) []string {
	prefix := "Gitverify-version: "

	versions := make([]string, 0)
	for i := len(messageLines) - 1; i >= 0; i-- {
		if strings.HasPrefix(messageLines[i], prefix) {
			version := strings.TrimPrefix(messageLines[i], prefix)
			versions = append(versions, version)
		}
	}

	return versions
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

	var tagHash *plumbing.Hash = nil
	var tagReference *plumbing.Reference = nil
	if opts.Tag != "" {
		tags, err := repo.Tags()
		if err != nil {
			return err
		}

		tagFound := false
		err = tags.ForEach(func(tag *plumbing.Reference) error {
			tagName := strings.TrimPrefix(tag.Name().String(), "refs/tags/")
			if tagName == opts.Tag {
				tagReference = tag
				if opts.Commit == "" {
					config.requireSignedTags = true
				}
				err := validateTag(tag, state, config, commitMetadata, gitHashSHA1, gitHashSHA512, !opts.InsecurePartialVerification, false)
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

	commit := opts.Commit
	if commit == "" {
		commit = tagHash.String()
	}

	var targetHash = plumbing.ZeroHash
	if commit != "" {
		c, found := state.CommitMap[plumbing.NewHash(commit)]
		if !found {
			return fmt.Errorf("target commit '%s' not found", commit)
		}

		if !opts.InsecurePartialVerification || opts.Commit != "" {
			err = validateCommitsRecursively(c, state, commitMetadata, gitHashSHA512, config, opts)
			if err != nil {
				return err
			}
		}

		targetHash = c.Hash

		if opts.VerifyAtHEAD {
			if targetHash != headHash {
				return fmt.Errorf("HEAD does not point to the target commit %s", commit)
			}
		}

		if tagHash != nil {
			if targetHash != *tagHash {
				return fmt.Errorf("target tag '%s' does not point to target commit %s ", opts.Tag, targetHash)
			}
		}
	}

	onProtectedBranch := false
	if opts.Branch != "" {
		remotes, err := repo.References()
		if err != nil {
			return err
		}

		branchFound := false
		err = remotes.ForEach(func(reference *plumbing.Reference) error {
			isProtected, branchName := IsProtected(reference, config)

			if branchName == opts.Branch {
				branchFound = true

				c, found := state.CommitMap[reference.Hash()]
				if !found {
					return fmt.Errorf("commit '%s' not found", reference.Hash().String())
				}

				if !opts.InsecurePartialVerification {
					if isProtected {
						err := validateProtectedBranch(reference, branchName, state, commitMetadata, config, gitHashSHA512)
						if err != nil {
							return fmt.Errorf("failed to validate protected branch '%s' rules: %w", reference.Name(), err)
						}
					} else {
						err := validateCommitsRecursively(c, state, commitMetadata, gitHashSHA512, config, opts)
						if err != nil {
							return err
						}
					}
				}

				if commit != "" && opts.VerifyAtTip {
					if targetHash != c.Hash {
						return fmt.Errorf("target commit %s does not point to the tip of branch '%s'", targetHash.String(), reference.Name())
					}
				} else {
					err = validateOnBranch(targetHash, branchName, c, state, commitMetadata, gitHashSHA512, config)
					if err != nil {
						return err
					}
				}

				if isProtected {
					onProtectedBranch = true
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

	if opts.Tag != "" {
		if config.requireTagsToBeOnProtectedBranches {
			if opts.Branch == "" {
				return fmt.Errorf("requireTagsBeOnProtectedBranches must be used with --branch")
			}

			isExempt, err := tagIsExempt(tagReference, state, config, gitHashSHA1, gitHashSHA512)
			if err != nil {
				return err
			}

			if !isExempt && !onProtectedBranch {
				return fmt.Errorf("requireTagsBeOnProtectedBranches set but tag %s is not on a protected branch", opts.Tag)
			}
		}
	}

	return nil
}

func validateOnBranch(targetHash plumbing.Hash, branchName string, c *object.Commit, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData, gitHashSHA512 githash.GitHash, config *RepoConfig) error {
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

		err := validateCommit(parent, state, commitMetadata, gitHashSHA512, config)
		if err != nil {
			return err
		}

		current = parent
	}

	return nil
}

func validateCommitsRecursively(c *object.Commit, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData, gitHashSHA512 githash.GitHash, config *RepoConfig, opts *ValidateOptions) error {
	err := validateCommit(c, state, commitMetadata, gitHashSHA512, config)
	if err != nil {
		return err
	}

	if opts.InsecurePartialVerification {
		err = verifyConnectedToAfter(c, state, commitMetadata)
		if err != nil {
			return err
		}

		return nil
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

				if !commitMetadata[parent.Hash].AfterOrAncestorOfAfter {
					err := validateCommit(parent, state, commitMetadata, gitHashSHA512, config)
					if err != nil {
						return err
					}

					queue = append(queue, parent)
					visited.Add(parentHash)
				}
			}

			if !config.verifyAllCommits {
				// Only check first parent
				break
			}
		}
	}

	return nil
}

func verifyConnectedToAfter(commit *object.Commit, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData) error {
	current := commit
	for {
		metadata, found := commitMetadata[current.Hash]
		if !found {
			return fmt.Errorf("commit %s not found in commit metadata", commit.Hash.String())
		}

		if metadata.AfterOrAncestorOfAfter || metadata.ConnectedToAfter {
			return nil
		}

		// optimistically assume that the chain will be verified
		metadata.ConnectedToAfter = true
		commitMetadata[current.Hash] = metadata

		if len(current.ParentHashes) == 0 {
			return fmt.Errorf("commit %s is not connected to any after", commit.Hash.String())
		}

		current, found = state.CommitMap[current.ParentHashes[0]]
		if !found {
			return fmt.Errorf("commit %s not found in commit metadata", commit.Hash.String())
		}
	}
}

func validateBranches(repo *git.Repository, state *gitkit.RepoState, commitMetadata map[plumbing.Hash]*CommitData, config *RepoConfig, gitHashSHA512 githash.GitHash) error {
	remotes, err := repo.References()
	if err != nil {
		return err
	}

	err = remotes.ForEach(func(reference *plumbing.Reference) error {
		isProtected, branchName := IsProtected(reference, config)

		if isProtected {
			err := validateProtectedBranch(reference, branchName, state, commitMetadata, config, gitHashSHA512)
			if err != nil {
				return err
			}
		} else {
			if config.verifyAllCommits {
				referenceName := reference.Name().String()
				if strings.HasPrefix(referenceName, "refs/tags/") {
					return nil
				}

				if reference.Type() == plumbing.HashReference {
					c, found := state.CommitMap[reference.Hash()]
					if !found {
						return fmt.Errorf("did not find commit %s for reference %s", reference.Hash().String(), referenceName)
					}

					err := verifyConnectedToAfter(c, state, commitMetadata)
					if err != nil {
						return err
					}
				}
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
		err := validateCommit(current, state, commitMetadata, gitHashSHA512, config)
		if err != nil {
			return err
		}

		if current.Hash == targetAfter {
			break
		}

		metadata, found := commitMetadata[current.Hash]
		if !found {
			return fmt.Errorf("commit %s not found in metadata", current.Hash.String())
		}

		if !metadata.OnProtectedBranch {
			// optimistically mark as on protected branch
			metadata.OnProtectedBranch = true
			commitMetadata[current.Hash] = metadata
		}

		if config.requireMergeCommits {
			if len(current.ParentHashes) != 2 {
				return fmt.Errorf("requireMergeCommits is set, but commit %s on protected branch has %d parents", current.Hash.String(), len(current.ParentHashes))
			}
		}

		if config.requireCountersigning {
			if metadata.MergeTag == nil {
				return fmt.Errorf("requireCountersigning is set, but no mergetag in commit %s", current.Hash.String())
			}
		}

		_, found = config.maintainerEmails[current.Committer.Email]
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

		current, found = state.CommitMap[current.ParentHashes[0]]
		if !found {
			return fmt.Errorf("did not find commit %s", reference.Hash().String())
		}
	}

	// mark after and prior commits to be on a protected branch
	for {
		metadata, found := commitMetadata[current.Hash]
		if !found {
			return fmt.Errorf("commit %s not found in metadata", current.Hash.String())
		}

		if metadata.OnProtectedBranch {
			break
		} else {
			metadata.OnProtectedBranch = true
			commitMetadata[current.Hash] = metadata
		}

		if len(current.ParentHashes) == 0 {
			break
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

func validateTags(repo *git.Repository, state *gitkit.RepoState, repoConfig *RepoConfig, commitMetadata map[plumbing.Hash]*CommitData, gitHashSHA1 githash.GitHash, gitHashSHA512 githash.GitHash) error {
	tags, err := repo.Tags()
	if err != nil {
		return err
	}

	err = tags.ForEach(func(tag *plumbing.Reference) error {
		return validateTag(tag, state, repoConfig, commitMetadata, gitHashSHA1, gitHashSHA512, true, true)
	})
	if err != nil {
		return err
	}

	return nil
}

func validateTag(tag *plumbing.Reference, state *gitkit.RepoState, repoConfig *RepoConfig, commitMetadata map[plumbing.Hash]*CommitData, gitHashSHA1 githash.GitHash, gitHashSHA512 githash.GitHash, fullVerification bool, onProtectedBranchVerification bool) error {
	tagReference := tag.Name().String()
	tagPrefix := "refs/tags/"
	if !strings.HasPrefix(tag.Name().String(), tagPrefix) {
		return fmt.Errorf("tag name %s does not start with %s", tag.Name().String(), tagPrefix)
	}
	tagName := strings.TrimPrefix(tag.Name().String(), "refs/tags/")

	isExempt, err := tagIsExempt(tag, state, repoConfig, gitHashSHA1, gitHashSHA512)
	if err != nil {
		return err
	}

	t, isAnnotatedTag := state.TagMap[tag.Hash()]
	if isAnnotatedTag {
		if tagName != t.Name {
			return fmt.Errorf("tag ref '%s' does not match name '%s'", tagName, t.Name)
		}

		if !isExempt {
			signatureType, err := inferSignatureType(t.PGPSignature)
			if err != nil {
				return err
			}

			id, found := repoConfig.maintainerEmails[t.Tagger.Email]
			if !found {
				return fmt.Errorf("no maintainer with email '%s' for tag %s", t.Tagger.Email, tagName)
			}

			switch signatureType {
			case SignatureTypeSSH:
				content, err := tagContent(t)
				if err != nil {
					return err
				}

				sshKeys := id.tagSSHPublicKeys
				if countersignTagRegex.MatchString(tagReference) {
					sshKeys = id.countersignTagSSHPublicKeys
				}

				err = validateSSH(content, t.PGPSignature, sshKeys, repoConfig)
				if err != nil {
					return fmt.Errorf("failed to validate tag %s: %w", tagName, err)
				}
			case SignatureTypePGP:
				pgpKeys := id.tagPGPPublicKeys
				if countersignTagRegex.MatchString(tagReference) {
					pgpKeys = id.countersignTagPGPPublicKeys
				}

				err := validateIdentityPGPTag(t, pgpKeys, repoConfig)
				if err != nil {
					return err
				}
			case SignatureTypeNone:
				if repoConfig.requireSignedTags {
					return fmt.Errorf("unsigned annotated tag: %s", tagName)
				}
			default:
				return fmt.Errorf("unknown signature type for tag: %s", tagName)
			}

			c, found := state.CommitMap[t.Target]
			if !found {
				return fmt.Errorf("commit %s missing for tag %s", t.Target.String(), tag.Hash().String())
			}

			err = verifyConnectedToAfter(c, state, commitMetadata)
			if err != nil {
				return err
			}

			if onProtectedBranchVerification {
				metadata, found := commitMetadata[c.Hash]
				if !found {
					return fmt.Errorf("commit %s missing in metadata", c.Hash.String())
				}

				err = verifyOnProtectedBranch(tagName, tagReference, metadata, repoConfig)
				if err != nil {
					return err
				}
			}

			if fullVerification {
				err := validateCommit(c, state, commitMetadata, gitHashSHA512, repoConfig)
				if err != nil {
					return err
				}

				err = verifyDistinctIdentities(t, c, commitMetadata, repoConfig)
				if err != nil {
					return err
				}

				if repoConfig.requireMatchedVersions {
					err = verifyVersionForTag(tagReference, tagName, c, commitMetadata)
					if err != nil {
						return err
					}
				}
			}
		}
	} else {
		if !isExempt {
			if repoConfig.requireSignedTags {
				return fmt.Errorf("tag '%s' is lightweight, but signing is required", tagName)
			}

			c, found := state.CommitMap[tag.Hash()]
			if !found {
				return fmt.Errorf("commit %s missing for tag %s", tag.Hash(), tagName)
			}

			err := verifyConnectedToAfter(c, state, commitMetadata)
			if err != nil {
				return err
			}

			if onProtectedBranchVerification {
				metadata, found := commitMetadata[c.Hash]
				if !found {
					return fmt.Errorf("commit %s missing in metadata", c.Hash.String())
				}

				err = verifyOnProtectedBranch(tagName, tagReference, metadata, repoConfig)
				if err != nil {
					return err
				}
			}

			if fullVerification {
				err := validateCommit(c, state, commitMetadata, gitHashSHA512, repoConfig)
				if err != nil {
					return err
				}

				if repoConfig.requireMatchedVersions {
					err = verifyVersionForTag(tagReference, tagName, c, commitMetadata)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func tagIsExempt(tag *plumbing.Reference, state *gitkit.RepoState, repoConfig *RepoConfig, gitHashSHA1 githash.GitHash, gitHashSHA512 githash.GitHash) (bool, error) {
	isExempt := false

	t, isAnnotatedTag := state.TagMap[tag.Hash()]
	if isAnnotatedTag {
		if t.Hash != tag.Hash() {
			return false, fmt.Errorf("inconsistent hash for tag %s", tag.Hash().String())
		}

		hash := t.Hash
		verifiedHash, err := gitHashSHA1.TagSum(hash)
		if err != nil {
			return false, err
		}

		if !bytes.Equal(verifiedHash, hash[:]) {
			return false, fmt.Errorf("failed to verify hash for annotated tag %s", tag.Hash().String())
		}
	} else {
		hash := tag.Hash()
		verifiedHash, err := gitHashSHA1.CommitSum(tag.Hash())
		if err != nil {
			return false, err
		}

		if !bytes.Equal(verifiedHash, hash[:]) {
			return false, fmt.Errorf("failed to verify hash for light weight tag %s", tag.Hash().String())
		}
	}

	tagHash, found := repoConfig.exemptedTags[tag.Name().String()]
	if found {
		if tagHash != tag.Hash().String() {
			return false, fmt.Errorf("wrong hash.sha1 for exempted tag '%s', got %s, expected %s", tag.Name().String(), tag.Hash().String(), tagHash)
		}
		isExempt = true
	}

	tagHashSHA512, found := repoConfig.exemptedTagsSHA512[tag.Name().String()]
	if found {
		var sha512Hash []byte
		var err error
		if isAnnotatedTag {
			sha512Hash, err = gitHashSHA512.TagSum(t.Hash)
			if err != nil {
				return false, err
			}
		} else {
			sha512Hash, err = gitHashSHA512.CommitSum(tag.Hash())
			if err != nil {
				return false, err
			}
		}

		h := hex.EncodeToString(sha512Hash)
		if tagHashSHA512 != h {
			return false, fmt.Errorf("wrong SHA-512 for exempted tag '%s', got %s, expected %s", tag.Name().String(), h, tagHashSHA512)
		}
		isExempt = true
	}

	return isExempt, nil
}

func verifyOnProtectedBranch(tagName string, tagReference string, metadata *CommitData, repoConfig *RepoConfig) error {
	if repoConfig.requireTagsToBeOnProtectedBranches && !metadata.OnProtectedBranch {
		if !countersignTagRegex.MatchString(tagReference) {
			return fmt.Errorf("requireTagsBeOnProtectedBranches set but tag %s is not on a protected branch", tagName)
		}
	}

	return nil
}

func verifyDistinctIdentities(tag *object.Tag, commit *object.Commit, commitMetadata map[plumbing.Hash]*CommitData, repoConfig *RepoConfig) error {
	metadata, found := commitMetadata[commit.Hash]
	if !found {
		return fmt.Errorf("commit %s not found in commit metadata", commit.Hash.String())
	}

	mergeTag := metadata.MergeTag
	if metadata.AfterOrAncestorOfAfter && commit.MergeTag != "" {
		var err error
		mergeTag, err = extractMergeTag(commit)
		if err != nil {
			return err
		}
	}

	taggerEmail := tag.Tagger.Email
	if mergeTag != nil {
		countersignCommitEmail := commit.Committer.Email
		countersignTagEmail := mergeTag.Tagger.Email
		if repoConfig.requireDistinctTagIdentities {
			if taggerEmail == countersignCommitEmail {
				return fmt.Errorf("requireDistinctTagIdentities is set but identity %s is reused for countersigned commit and tag %s", taggerEmail, tag.Name)
			}

			if taggerEmail == countersignTagEmail {
				return fmt.Errorf("requireDistinctTagIdentities is set but identity %s is reused for countersigned tag and tag %s", taggerEmail, tag.Name)
			}
		}

		if repoConfig.requireDistinctCountersignTagIdentities {
			if taggerEmail == countersignTagEmail {
				return fmt.Errorf("requireDistinctCountersignTagIdentities is set but identity %s is reused for countersigned commit and tag %s", taggerEmail, tag.Name)
			}
		}

		if repoConfig.requireDistinctCountersignCommitIdentities {
			if taggerEmail == countersignCommitEmail {
				return fmt.Errorf("requireDistinctCountersignCommitIdentities is set but identity %s is reused for countersigned commit and tag %s", taggerEmail, tag.Name)
			}
		}
	} else {
		commitEmail := commit.Committer.Email
		if repoConfig.requireDistinctTagIdentities {
			if taggerEmail == commitEmail {
				return fmt.Errorf("requireDistinctTagIdentities is set but identity %s is reused for tag %s", taggerEmail, tag.Name)
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
			commitMap[hash] = &CommitData{}
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

func IsProtected(reference *plumbing.Reference, config *RepoConfig) (bool, string) {
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
