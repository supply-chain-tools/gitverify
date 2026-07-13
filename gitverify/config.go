package gitverify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/supply-chain-tools/go-sandbox/hashset"
)

// TODO improve
const tagRefRegex = "^refs/tags/.+$"

type Config struct {
	Type              string         `json:"_type"`
	Identities        []Identity     `json:"identities"`
	ForgeIdentity     *ForgeIdentity `json:"forgeIdentity"`
	Maintainers       []string       `json:"maintainers"`
	Contributors      []string       `json:"contributors"`
	Rules             *Rules         `json:"rules"`
	ProtectedBranches []string       `json:"protectedBranches"`

	Repositories []Repository `json:"repositories"`
}

type Identity struct {
	Email         string   `json:"email"`
	SSHPublicKeys []SSHKey `json:"sshPublicKeys"`
	PGPPublicKeys []PGPKey `json:"pgpPublicKeys"`
	ForgeUsername *string  `json:"forgeUsername"`
	ForgeUserId   *string  `json:"forgeUserId"`
}

type SSHKey struct {
	Key string `json:"key"`
	KeyOptions
}

type PGPKey struct {
	Key string `json:"key"`
	KeyOptions
}

type KeyOptions struct {
	SignCommits            *bool `json:"signCommits"`
	SignTags               *bool `json:"signTags"`
	SignCountersignTags    *bool `json:"signCountersignTags"`
	SignCountersignCommits *bool `json:"signCountersignCommits"`
}

type ParsedIdentity struct {
	Email         string
	SSHPublicKeys []ParsedSSHKey
	PGPPublicKeys []ParsedPGPKey
	ForgeUsername *string
	ForgeUserId   *string
}

type ParsedSSHKey struct {
	Key string
	ParsedKeyOptions
}

type ParsedPGPKey struct {
	Key string
	ParsedKeyOptions
}

type ParsedKeyOptions struct {
	SignCommits            bool
	SignTags               bool
	SignCountersignTags    bool
	SignCountersignCommits bool
}

type Tag struct {
	PGPPublicKeys []string `json:"pgpPublicKeys"`
	SSHPublicKeys []string `json:"sshPublicKeys"`
}

type Countersign struct {
	PGPPublicKeys []string `json:"pgpPublicKeys"`
	SSHPublicKeys []string `json:"sshPublicKeys"`
}

type ForgeIdentity struct {
	Email         string   `json:"email"`
	PGPPublicKeys []string `json:"pgpPublicKeys"`
	SSHPublicKeys []string `json:"sshPublicKeys"`
}

type Rules struct {
	AllowSSHSignatures     *bool `json:"allowSshSignatures"`
	RequireSSHUserPresent  *bool `json:"requireSshUserPresent"`
	RequireSSHUserVerified *bool `json:"requireSshUserVerified"`
	AllowSSHSHA256         *bool `json:"allowSshSha256"`

	AllowPGPSignatures *bool `json:"allowPGPSignatures"`

	RequireSignedTags     *bool `json:"requireSignedTags"`
	RequireMergeCommits   *bool `json:"requireMergeCommits"`
	RequireCountersigning *bool `json:"requireCountersigning"`

	RequireSHA512    *bool `json:"requireSha512"`
	VerifyAllCommits *bool `json:"verifyAllCommits"`

	TrustForge *bool `json:"trustForge"`

	RequireDedicatedTagKeys               *bool `json:"requireDedicatedTagKeys"`
	RequireDedicatedCountersignTagKeys    *bool `json:"requireDedicatedCountersignTagKeys"`
	RequireDedicatedCountersignCommitKeys *bool `json:"requireDedicatedCountersignCommitKeys"`

	RequireDistinctTagIdentities               *bool `json:"requireDistinctTagIdentities"`
	RequireDistinctCountersignTagsIdentities   *bool `json:"requireDistinctCountersignTagIdentities"`
	RequireDistinctCountersignCommitIdentities *bool `json:"requireDistinctCountersignCommitIdentities"`
}

type Repository struct {
	Uri   string  `json:"uri"`
	After []After `json:"after"`

	Maintainers       []string `json:"maintainers"`
	Contributors      []string `json:"contributors"`
	Rules             *Rules   `json:"rules"`
	ProtectedBranches []string `json:"protectedBranches"`

	ExemptTags []ExemptTag `json:"exemptTags"`
}

type Digests struct {
	SHA1   *string `json:"sha1,omitempty"`
	SHA512 *string `json:"sha512,omitempty"`
}

type After struct {
	SHA1   *string `json:"sha1,omitempty"`
	SHA512 *string `json:"sha512,omitempty"`
	Branch *string `json:"branch,omitempty"`
}

type ParsedConfig struct {
	Repositories []ParsedRepository
}

type ParsedRepository struct {
	Uri   string
	After []After

	Identities        []ParsedIdentity
	Maintainers       hashset.Set[string]
	Contributors      hashset.Set[string]
	Rules             ParsedRules
	ProtectedBranches hashset.Set[string]

	ExemptedTags []ExemptTag

	TrustedForge *ParsedForge
}

type ParsedForge struct {
	Email        string
	PGPPublicKey *string
	SSHPublicKey *string
}

type ParsedRules struct {
	AllowSSHSignatures     bool
	RequireSSHUserPresent  bool
	RequireSSHUserVerified bool
	AllowSSHSHA256         bool

	AllowPGPSignatures bool

	RequireSignedTags     bool
	RequireMergeCommits   bool
	RequireCountersigning bool

	RequireSHA512    bool
	VerifyAllCommits bool

	TrustForge bool

	RequireDedicatedTagKeys               bool
	RequireDedicatedCountersignTagKeys    bool
	RequireDedicatedCountersignCommitKeys bool

	RequireDistinctTagIdentities               bool
	RequireDistinctCountersignTagIdentities    bool
	RequireDistinctCountersignCommitIdentities bool
}

func GetConfigPath(forge string, org string) (string, error) {
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDirectory, ".config", "gitverify", forge, org, "gitverify.json"), nil
}

func LoadConfig(configPath string) (*ParsedConfig, error) {
	if configPath == "" {
		return nil, fmt.Errorf("empty config path")
	}

	var p string
	if strings.HasPrefix(configPath, "/") {
		p = configPath
	} else if strings.HasPrefix(configPath, "~") {
		return nil, fmt.Errorf("~ not supported in config file path: %s", configPath)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}

		p = filepath.Join(cwd, configPath)
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", p, err)
	}

	config := &Config{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	err = dec.Decode(config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", p, err)
	}

	parsed, err := parseConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", p, err)
	}

	return parsed, nil
}

func parseConfig(config *Config) (*ParsedConfig, error) {
	prefix := "https://supply-chain-tools.github.io/schemas/gitverify/"
	if !strings.HasPrefix(config.Type, prefix) {
		return nil, fmt.Errorf("unsupported schema %s, expect %s", config.Type, prefix)
	}

	version := config.Type[len(prefix):]

	expectedVersion := "v0.3"
	if version != expectedVersion {
		return nil, fmt.Errorf("got schema version %s, expected %s", version, expectedVersion)
	}

	allURIs := hashset.New[string]()
	parsedRepos := make([]ParsedRepository, 0)

	var forge *ParsedForge = nil
	if config.ForgeIdentity != nil {
		numKeys := len(config.ForgeIdentity.PGPPublicKeys) + len(config.ForgeIdentity.SSHPublicKeys)
		if numKeys != 1 {
			return nil, fmt.Errorf("expected exactly one forge key, got %d", numKeys)
		}

		forge = &ParsedForge{
			Email: config.ForgeIdentity.Email,
		}

		if len(config.ForgeIdentity.PGPPublicKeys) > 0 {
			if config.ForgeIdentity.PGPPublicKeys[0] == "" {
				return nil, fmt.Errorf("forge PGP key must be non-empty")
			}

			forge.PGPPublicKey = &config.ForgeIdentity.PGPPublicKeys[0]
		}

		if len(config.ForgeIdentity.SSHPublicKeys) > 0 {
			if config.ForgeIdentity.SSHPublicKeys[0] == "" {
				return nil, fmt.Errorf("forge SSH key must be non-empty")
			}

			forge.SSHPublicKey = &config.ForgeIdentity.SSHPublicKeys[0]
		}
	} else {
		if config.Rules.TrustForge != nil && *config.Rules.TrustForge {
			return nil, fmt.Errorf("trustForge is set globally but no forgeIdentity specified ")
		}
	}

	for _, repo := range config.Repositories {
		uri, err := validateUri(repo.Uri)
		if err != nil {
			return nil, err
		}

		defaultKeyOptions := ParsedKeyOptions{
			SignCommits:            true,
			SignTags:               true,
			SignCountersignTags:    false,
			SignCountersignCommits: false,
		}

		identities := parseIdentities(config.Identities, defaultKeyOptions)

		maintainers, err := combineMaintainers(config.Maintainers, repo.Maintainers)
		if err != nil {
			return nil, err
		}

		contributors, err := combineContributors(config.Contributors, repo.Contributors)
		if err != nil {
			return nil, err
		}

		err = ensurePresent(identities, maintainers, contributors)
		if err != nil {
			return nil, err
		}

		defaultRules := ParsedRules{
			AllowSSHSignatures:                         true,
			RequireSSHUserPresent:                      false,
			RequireSSHUserVerified:                     false,
			AllowSSHSHA256:                             false,
			AllowPGPSignatures:                         true,
			RequireSignedTags:                          true,
			RequireMergeCommits:                        true,
			RequireCountersigning:                      false,
			RequireSHA512:                              false,
			VerifyAllCommits:                           false,
			TrustForge:                                 false,
			RequireDedicatedTagKeys:                    false,
			RequireDedicatedCountersignTagKeys:         false,
			RequireDedicatedCountersignCommitKeys:      false,
			RequireDistinctTagIdentities:               false,
			RequireDistinctCountersignTagIdentities:    true,
			RequireDistinctCountersignCommitIdentities: true,
		}

		parsedRules, err := combineRules(defaultRules, config.Rules, repo.Rules)
		if err != nil {
			return nil, err
		}

		var trustedForge *ParsedForge = nil
		if parsedRules.TrustForge == true {
			if forge == nil {
				return nil, fmt.Errorf("trustForge is set, but no forgeIdentity specified")
			}

			if forge.PGPPublicKey != nil && parsedRules.AllowPGPSignatures == false {
				return nil, fmt.Errorf("forgeIdentity.pgpPublicKeys is specified, but allowPGPSignatures is false")
			}

			if forge.SSHPublicKey != nil && parsedRules.AllowSSHSignatures == false {
				return nil, fmt.Errorf("forgeIdentity.sshPublicKeys is specified, but allowSSHSignatures is false")
			}

			trustedForge = forge
		}

		after, err := validateAfter(repo.After, parsedRules.RequireSHA512)
		if err != nil {
			return nil, err
		}

		exemptTags, err := validateExemptTags(repo.ExemptTags, parsedRules.RequireSHA512)
		if err != nil {
			return nil, err
		}

		protectedBranches, err := combineProtectedBranches(config.ProtectedBranches, repo.ProtectedBranches)
		if err != nil {
			return nil, err
		}

		if protectedBranches.Size() == 0 {
			return nil, fmt.Errorf("at least one protected branch must be specified")
		}

		if parsedRules.RequireCountersigning == true && parsedRules.RequireMergeCommits == false {
			return nil, fmt.Errorf("requireCountersigning can only be used with requireMergeCommits")
		}

		if parsedRules.RequireCountersigning == true && parsedRules.TrustForge {
			return nil, fmt.Errorf("trustForge cannot be used with requireCountersigning")
		}

		if parsedRules.RequireSHA512 == true && parsedRules.RequireCountersigning == false {
			return nil, fmt.Errorf("requireSha512 can only be used with requireCountersigning")
		}

		if parsedRules.RequireSHA512 == true && parsedRules.AllowSSHSHA256 == true {
			return nil, fmt.Errorf("allowSshSha256 cannot be used with requireSha512")
		}

		if parsedRules.RequireSHA512 == true && parsedRules.AllowPGPSignatures == true {
			return nil, fmt.Errorf("requireSha512 does not currently support allowPgpSignatures")
		}

		if parsedRules.RequireDistinctTagIdentities == true && parsedRules.RequireSignedTags == false {
			return nil, fmt.Errorf("requireSignedTags must be used with requireDistinctTagIdentities")
		}

		if !(parsedRules.RequireDistinctCountersignTagIdentities ||
			parsedRules.RequireDistinctCountersignCommitIdentities ||
			parsedRules.RequireDedicatedCountersignTagKeys ||
			parsedRules.RequireDedicatedCountersignCommitKeys) {
			return nil, fmt.Errorf("when using countersigning, dedicated keys or distinct identities must be set")
		}

		if allURIs.Contains(uri) {
			return nil, fmt.Errorf("duplicate URI '%s'", uri)
		}
		allURIs.Add(uri)

		parsedRepos = append(parsedRepos, ParsedRepository{
			Uri:               uri,
			After:             after,
			Identities:        identities,
			Maintainers:       maintainers,
			Contributors:      contributors,
			Rules:             parsedRules,
			ProtectedBranches: protectedBranches,
			ExemptedTags:      exemptTags,
			TrustedForge:      trustedForge,
		})
	}

	parsedConfig := ParsedConfig{
		Repositories: parsedRepos,
	}

	return &parsedConfig, nil
}

func parseIdentities(identities []Identity, defaultKeyOptions ParsedKeyOptions) []ParsedIdentity {
	result := make([]ParsedIdentity, 0)

	for _, identity := range identities {
		sshKeys := make([]ParsedSSHKey, 0)
		for _, sshKey := range identity.SSHPublicKeys {
			sshKeys = append(sshKeys, ParsedSSHKey{
				Key:              sshKey.Key,
				ParsedKeyOptions: combineKeyOptions(sshKey.KeyOptions, defaultKeyOptions),
			})
		}

		pgpKeys := make([]ParsedPGPKey, 0)
		for _, pgpKey := range identity.PGPPublicKeys {
			pgpKeys = append(pgpKeys, ParsedPGPKey{
				Key:              pgpKey.Key,
				ParsedKeyOptions: combineKeyOptions(pgpKey.KeyOptions, defaultKeyOptions),
			})
		}

		result = append(result, ParsedIdentity{
			Email:         identity.Email,
			SSHPublicKeys: sshKeys,
			PGPPublicKeys: pgpKeys,
			ForgeUsername: identity.ForgeUsername,
			ForgeUserId:   identity.ForgeUserId,
		})
	}

	return result
}

func combineKeyOptions(options KeyOptions, defaults ParsedKeyOptions) ParsedKeyOptions {
	result := defaults

	if options.SignCommits != nil {
		result.SignCommits = *options.SignCommits
	}

	if options.SignTags != nil {
		result.SignTags = *options.SignTags
	}

	if options.SignCountersignTags != nil {
		result.SignCountersignTags = *options.SignCountersignTags
	}

	if options.SignCountersignCommits != nil {
		result.SignCountersignCommits = *options.SignCountersignCommits
	}

	return result
}

func ensurePresent(identities []ParsedIdentity, maintainers hashset.Set[string], contributors hashset.Set[string]) error {
	identityEmails := hashset.New[string]()

	for _, identity := range identities {
		identityEmails.Add(identity.Email)
	}

	maintainerDiff := maintainers.Difference(identityEmails)
	contributorDiff := contributors.Difference(identityEmails)

	if maintainerDiff.Size() > 0 {
		return fmt.Errorf("maintainers '%s' not present in identities", strings.Join(maintainerDiff.Values(), ","))
	}

	if contributorDiff.Size() > 0 {
		return fmt.Errorf("contributors '%s' not present in identities", strings.Join(contributorDiff.Values(), ","))
	}

	return nil
}

func validateUri(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", err
	}

	// https://spdx.github.io/spdx-spec/v2.3/package-information/#77-package-download-location-field
	gitHTTPS := "git+https"
	gitSSH := "git+ssh"
	if !(u.Scheme == gitHTTPS || u.Scheme == gitSSH) {
		return "", fmt.Errorf("got scheme '%s' for repo uri '%s', expected '%s' or '%s'", u.Scheme, uri, gitHTTPS, gitSSH)
	}

	ext := path.Ext(u.Path)
	if ext != ".git" {
		return "", fmt.Errorf("got extension '%s' for repo uri '%s', expected '.git'", ext, uri)
	}

	return uri, nil
}

func validateAfter(after []After, requireSHA512 bool) ([]After, error) {
	// TODO consider verifying after to be globally unique
	allBranches := hashset.New[string]()
	allSHA1 := hashset.New[string]()
	allSHA512 := hashset.New[string]()

	for _, a := range after {
		if a.SHA1 == nil && a.SHA512 == nil {
			return nil, fmt.Errorf("either after.sha1 or after.sha512 must be set, or both")
		}

		if a.SHA1 != nil {

			if !HexSHA1Regex.MatchString(*a.SHA1) {
				return nil, fmt.Errorf("after.sha1 '%s' must be a 40 character hex", *a.SHA1)
			}

			if allSHA1.Contains(*a.SHA1) {
				return nil, fmt.Errorf("after SHA1 '%s' must be unique", *a.SHA1)
			}

			allSHA1.Add(*a.SHA1)
		}

		if requireSHA512 && a.SHA512 == nil {
			return nil, fmt.Errorf("after.sha512 is missing, but requireSha512 is set")
		}

		if a.SHA512 != nil {
			if !HexSHA512Regex.MatchString(*a.SHA512) {
				return nil, fmt.Errorf("after.sha512 '%s' must be a 128 character hex", *a.SHA512)
			}

			if allSHA512.Contains(*a.SHA512) {
				return nil, fmt.Errorf("after.sha512 '%s' must be unique", *a.SHA512)
			}

			allSHA512.Add(*a.SHA512)
		}

		if a.Branch != nil {
			if allBranches.Contains(*a.Branch) {
				return nil, fmt.Errorf("duplicate branch '%s'", *a.Branch)
			}
			allBranches.Add(*a.Branch)
		}
	}

	return after, nil
}

func validateExemptTags(exemptTags []ExemptTag, requireSHA512 bool) ([]ExemptTag, error) {
	allTagRefs := hashset.New[string]()

	for _, exemptTag := range exemptTags {
		match, err := regexp.MatchString(tagRefRegex, exemptTag.Ref)
		if err != nil {
			return nil, err
		}

		if !match {
			return nil, fmt.Errorf("invalid exemptTag.ref '%s'", exemptTag.Ref)
		}

		if exemptTag.Hash.SHA1 == nil && exemptTag.Hash.SHA512 == nil {
			return nil, fmt.Errorf("either exemptTag.hash.sha1 or exemptTag.hash.sha512 must be set, or both")
		}

		if exemptTag.Hash.SHA1 != nil {
			if !HexSHA1Regex.MatchString(*exemptTag.Hash.SHA1) {
				return nil, fmt.Errorf("exemptTag.hash.sha1 '%s' must be a 40 character hex", *exemptTag.Hash.SHA1)
			}
		}

		if requireSHA512 && exemptTag.Hash.SHA512 == nil {
			return nil, fmt.Errorf("exemptTag.hash.sha512 is missing, but requireSha512 is set")
		}

		if exemptTag.Hash.SHA512 != nil {
			if !HexSHA512Regex.MatchString(*exemptTag.Hash.SHA512) {
				return nil, fmt.Errorf("exemptTag.hash.sha512 '%s' must be a 128 character hex", *exemptTag.Hash.SHA512)
			}
		}

		if allTagRefs.Contains(exemptTag.Ref) {
			return nil, fmt.Errorf("duplicate exemptTag.ref '%s'", exemptTag.Ref)
		}
		allTagRefs.Add(exemptTag.Ref)
	}

	return exemptTags, nil
}

func combineMaintainers(global []string, local []string) (hashset.Set[string], error) {
	result := hashset.New[string](global...)
	result.Add(local...)

	return result, nil
}

func combineContributors(global []string, local []string) (hashset.Set[string], error) {
	result := hashset.New[string](global...)
	result.Add(local...)

	return result, nil
}

func combineRules(defaultRules ParsedRules, global *Rules, local *Rules) (ParsedRules, error) {
	rules := overwriteExisting(defaultRules, global)
	rules = overwriteExisting(rules, local)

	return rules, nil
}

func overwriteExisting(existing ParsedRules, rules *Rules) ParsedRules {
	if rules != nil {
		if rules.AllowSSHSignatures != nil {
			existing.AllowSSHSignatures = *rules.AllowSSHSignatures
		}

		if rules.RequireSSHUserPresent != nil {
			existing.RequireSSHUserPresent = *rules.RequireSSHUserPresent
		}

		if rules.RequireSSHUserVerified != nil {
			existing.RequireSSHUserVerified = *rules.RequireSSHUserVerified
		}

		if rules.AllowSSHSHA256 != nil {
			existing.AllowSSHSHA256 = *rules.AllowSSHSHA256
		}

		if rules.AllowPGPSignatures != nil {
			existing.AllowPGPSignatures = *rules.AllowPGPSignatures
		}

		if rules.RequireSignedTags != nil {
			existing.RequireSignedTags = *rules.RequireSignedTags
		}

		if rules.RequireMergeCommits != nil {
			existing.RequireMergeCommits = *rules.RequireMergeCommits
		}

		if rules.RequireCountersigning != nil {
			existing.RequireCountersigning = *rules.RequireCountersigning
		}

		if rules.RequireSHA512 != nil {
			existing.RequireSHA512 = *rules.RequireSHA512
		}

		if rules.VerifyAllCommits != nil {
			existing.VerifyAllCommits = *rules.VerifyAllCommits
		}

		if rules.TrustForge != nil {
			existing.TrustForge = *rules.TrustForge
		}

		if rules.RequireDedicatedTagKeys != nil {
			existing.RequireDedicatedTagKeys = *rules.RequireDedicatedTagKeys
		}

		if rules.RequireDedicatedCountersignTagKeys != nil {
			existing.RequireDedicatedCountersignTagKeys = *rules.RequireDedicatedCountersignTagKeys
		}

		if rules.RequireDedicatedCountersignCommitKeys != nil {
			existing.RequireDedicatedCountersignCommitKeys = *rules.RequireDedicatedCountersignCommitKeys
		}

		if rules.RequireDistinctTagIdentities != nil {
			existing.RequireDistinctTagIdentities = *rules.RequireDistinctTagIdentities
		}

		if rules.RequireDistinctCountersignTagsIdentities != nil {
			existing.RequireDistinctCountersignTagIdentities = *rules.RequireDistinctCountersignTagsIdentities
		}

		if rules.RequireDistinctCountersignCommitIdentities != nil {
			existing.RequireDistinctCountersignCommitIdentities = *rules.RequireDistinctCountersignCommitIdentities
		}
	}

	return existing
}

func combineProtectedBranches(global []string, local []string) (hashset.Set[string], error) {
	result := hashset.New[string](global...)
	result.Add(local...)

	return result, nil
}
