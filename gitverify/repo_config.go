package gitverify

import (
	"encoding/hex"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/supply-chain-tools/go-sandbox/hashset"
	"golang.org/x/crypto/ssh"
)

type RepoConfig struct {
	afterSHA1                                  hashset.Set[plumbing.Hash]
	afterSHA512                                hashset.Set[[64]byte]
	sha1ToBranch                               map[plumbing.Hash]string
	branchToSHA1                               map[string]plumbing.Hash
	sha512ToBranch                             map[[64]byte]string
	afterSHA1ToSHA512                          map[plumbing.Hash][64]byte
	maintainerEmails                           map[string]identity
	maintainerOrContributorEmails              map[string]identity
	maintainerForgeEmails                      map[string]identity
	maintainerOrContributorForgeEmails         map[string]identity
	trustedForge                               *forge
	allowSSHSignatures                         bool
	requireSSHUserPresent                      bool
	requireSSHUserVerified                     bool
	allowSSHSHA256                             bool
	allowPGPSignatures                         bool
	requireSignedTags                          bool
	requireMergeCommits                        bool
	requireCountersigning                      bool
	requireSHA512                              bool
	protectedBranches                          hashset.Set[string]
	exemptedTags                               map[string]string
	exemptedTagsSHA512                         map[string]string
	lockdown                                   bool
	requireDistinctTagIdentities               bool
	requireDistinctCountersignCommitIdentities bool
	requireDistinctCountersignTagIdentities    bool
}

type identity struct {
	email                    string
	forgeUsername            *string
	forgeUserId              *string
	sshPublicKeys            map[string]*ssh.PublicKey
	tagSSHPublicKeys         map[string]*ssh.PublicKey
	counterSignPublicKeys    map[string]*ssh.PublicKey
	pgpPublicKeys            []string
	tagPGPPublicKeys         []string
	countersignPGPPublicKeys []string
}

type forge struct {
	email        string
	pgpPublicKey *string
	identity     *identity
}

func LoadRepoConfig(config *ParsedConfig, repoUri string) (*RepoConfig, error) {
	var repo *ParsedRepository = nil
	for _, r := range config.Repositories {
		if r.Uri == repoUri {
			repo = &r
		}
	}

	if repo == nil {
		return nil, fmt.Errorf("repository %s not found in config", repoUri)
	}

	if repo.Maintainers.Size() == 0 {
		return nil, fmt.Errorf("no maintainers specified: %s", repoUri)
	}

	for _, m := range repo.Maintainers.Values() {
		if repo.Contributors.Contains(m) {
			return nil, fmt.Errorf("'%s' must be maintainer or contributor not both", m)
		}
	}

	allEmails := hashset.New[string]()
	maintainerEmails := make(map[string]identity)
	contributorEmails := make(map[string]identity)
	maintainerOrContributor := make(map[string]identity)

	allForgeEmails := hashset.New[string]()
	maintainerForgeEmails := make(map[string]identity)
	contributorForgeEmails := make(map[string]identity)
	maintainerOrContributorForgeEmails := make(map[string]identity)

	for _, i := range repo.Identities {
		sshCommitList := make([]string, 0)
		sshTagList := make([]string, 0)
		sshCountersignTagList := make([]string, 0)
		sshCountersignCommitList := make([]string, 0)

		// TODO use one map for all keys, check type before use and improve erro message
		for _, sshKey := range i.SSHPublicKeys {
			if sshKey.SignCommits {
				sshCommitList = append(sshCommitList, sshKey.Key)
			}

			if sshKey.SignTags {
				if repo.Rules.RequireDedicatedTagKeys {
					if sshKey.SignCommits || sshKey.SignCountersignTags || sshKey.SignCountersignCommits {
						return nil, fmt.Errorf("requireDedicatedTagKeys set, but key has other purposes")
					}
				}

				sshTagList = append(sshTagList, sshKey.Key)
			}

			if sshKey.SignCountersignTags {
				if repo.Rules.RequireDedicatedCountersignTagKeys {
					if sshKey.SignCommits || sshKey.SignTags || sshKey.SignCountersignCommits {
						return nil, fmt.Errorf("requireDedicatedCountersignTagKeys set, but key has other purposes")
					}
				}

				sshCountersignTagList = append(sshCountersignTagList, sshKey.Key)
			}

			if sshKey.SignCountersignCommits {
				if repo.Rules.RequireDedicatedCountersignCommitKeys {
					if sshKey.SignCommits || sshKey.SignTags || sshKey.SignCountersignTags {
						return nil, fmt.Errorf("requireDedicatedCountersignCommitKeys set, but key has other purposes")
					}
				}

				sshCountersignCommitList = append(sshCountersignCommitList, sshKey.Key)
			}
		}

		sshCommitMap, err := createSSSHPublicKeyMap(sshCommitList)
		if err != nil {
			return nil, err
		}

		sshTagMap, err := createSSSHPublicKeyMap(sshTagList)
		if err != nil {
			return nil, err
		}

		sshCountersignCommitMap, err := createSSSHPublicKeyMap(sshCountersignCommitList)
		if err != nil {
			return nil, err
		}

		pgpCommitList := make([]string, 0)
		pgpTagList := make([]string, 0)
		pgpCountersignCommitList := make([]string, 0)
		pgpCountersignTagList := make([]string, 0)

		// TODO add check of PGP keys
		for _, pgpKey := range i.PGPPublicKeys {
			if pgpKey.SignCommits {
				pgpCommitList = append(pgpCommitList, pgpKey.Key)
			}

			if pgpKey.SignTags {
				if repo.Rules.RequireDedicatedTagKeys {
					if pgpKey.SignCommits || pgpKey.SignCountersignTags || pgpKey.SignCountersignCommits {
						return nil, fmt.Errorf("requireDedicatedTagKeys set, but key has other purposes")
					}
				}

				pgpTagList = append(pgpTagList, pgpKey.Key)
			}

			if pgpKey.SignCountersignTags {
				if repo.Rules.RequireDedicatedCountersignTagKeys {
					if pgpKey.SignCommits || pgpKey.SignTags || pgpKey.SignCountersignCommits {
						return nil, fmt.Errorf("requireDedicatedCountersignTagKeys set, but key has other purposes")
					}
				}

				pgpCountersignTagList = append(pgpCountersignTagList, pgpKey.Key)
			}

			if pgpKey.SignCountersignCommits {
				if repo.Rules.RequireDedicatedCountersignCommitKeys {
					if pgpKey.SignCommits || pgpKey.SignTags || pgpKey.SignCountersignTags {
						return nil, fmt.Errorf("requireDedicatedCountersignCommitKeys set, but key has other purposes")
					}
				}

				pgpCountersignCommitList = append(pgpCountersignCommitList, pgpKey.Key)
			}
		}

		if len(pgpCommitList) > 1 {
			return nil, fmt.Errorf("only one PGP key is supported for each signing type, got %d SignCommits", len(pgpCommitList))
		}

		if len(pgpTagList) > 1 {
			return nil, fmt.Errorf("only one PGP key is supported for each signing type, got %d SignTags", len(pgpTagList))
		}

		if len(pgpCountersignTagList) > 1 {
			return nil, fmt.Errorf("only one PGP key is supported for each signing type, got %d CountersignTagList", len(pgpCountersignTagList))
		}

		if len(pgpCountersignCommitList) > 1 {
			return nil, fmt.Errorf("only one PGP key is supported for each signing type, got %d CountersignCommitList", len(pgpCountersignCommitList))
		}

		identityEntry := identity{
			email:                    i.Email,
			forgeUsername:            i.ForgeUsername,
			forgeUserId:              i.ForgeUserId,
			sshPublicKeys:            sshCommitMap,
			tagSSHPublicKeys:         sshTagMap,
			counterSignPublicKeys:    sshCountersignCommitMap,
			pgpPublicKeys:            pgpCommitList,
			tagPGPPublicKeys:         pgpTagList,
			countersignPGPPublicKeys: pgpCountersignCommitList,
		}

		var forgeEmail = ""
		if repo.TrustedForge != nil && repo.TrustedForge.Email == gitHubEmail && i.ForgeUsername != nil && i.ForgeUserId != nil {
			forgeEmail = gitHubUserEmail(*i.ForgeUserId, *i.ForgeUsername)

			if allForgeEmails.Contains(forgeEmail) {
				return nil, fmt.Errorf("duplicate forge email '%s' in repository %s", forgeEmail, repoUri)
			}
			allForgeEmails.Add(forgeEmail)
		}

		if allEmails.Contains(i.Email) {
			return nil, fmt.Errorf("duplicate email %s found in repository %s", i.Email, repoUri)
		}
		allEmails.Add(i.Email)

		if repo.Maintainers.Contains(i.Email) || repo.Contributors.Contains(i.Email) {
			maintainerOrContributor[i.Email] = identityEntry

			if forgeEmail != "" {
				maintainerOrContributorForgeEmails[forgeEmail] = identityEntry
			}
		}

		if repo.Maintainers.Contains(i.Email) {
			maintainerEmails[i.Email] = identityEntry

			if forgeEmail != "" {
				maintainerForgeEmails[forgeEmail] = identityEntry
			}
		}

		if repo.Contributors.Contains(i.Email) {
			contributorEmails[i.Email] = identityEntry

			if forgeEmail != "" {
				contributorForgeEmails[forgeEmail] = identityEntry
			}
		}
	}

	var f *forge
	if repo.TrustedForge != nil {
		f = &forge{
			email:        repo.TrustedForge.Email,
			pgpPublicKey: repo.TrustedForge.PGPPublicKey,
		}

		if repo.TrustedForge.SSHPublicKey != nil {
			sshPublicKeys, err := createSSSHPublicKeyMap([]string{*repo.TrustedForge.SSHPublicKey})
			if err != nil {
				return nil, err
			}

			f.identity = &identity{
				email:         repo.TrustedForge.Email,
				sshPublicKeys: sshPublicKeys,
			}
		}

	}

	exemptedTagMap := make(map[string]string)
	exemptedTagSHA512Map := make(map[string]string)
	for _, exemptTag := range repo.ExemptedTags {
		_, found := exemptedTagMap[exemptTag.Ref]
		if found {
			return nil, fmt.Errorf("duplicate exempted tag %s found in repository %s", exemptTag.Ref, repoUri)
		}

		_, found = exemptedTagSHA512Map[exemptTag.Ref]
		if found {
			return nil, fmt.Errorf("duplicate exempted SHA-512 tag %s found in repository %s", exemptTag.Ref, repoUri)
		}

		if exemptTag.Hash.SHA1 == nil && exemptTag.Hash.SHA512 == nil {
			return nil, fmt.Errorf("at least one of hash.sha1 and hash.sha512 must be set for exempted tag %s", exemptTag.Ref)
		}

		if exemptTag.Hash.SHA1 != nil {
			if !HexSHA1Regex.MatchString(*exemptTag.Hash.SHA1) {
				return nil, fmt.Errorf("SHA-1 hash for exempted tag must be 40 character hex, got %s", *exemptTag.Hash.SHA1)
			}
			exemptedTagMap[exemptTag.Ref] = *exemptTag.Hash.SHA1
		}

		if exemptTag.Hash.SHA512 != nil {
			if !HexSHA512Regex.MatchString(*exemptTag.Hash.SHA512) {
				return nil, fmt.Errorf("hash.sha512 for exempted tag must be 128 character hex, got %s", *exemptTag.Hash.SHA512)
			}

			exemptedTagSHA512Map[exemptTag.Ref] = *exemptTag.Hash.SHA512
		}
	}

	var afterSHA1 = hashset.New[plumbing.Hash]()
	var afterSHA512 = hashset.New[[64]byte]()
	afterSHA1ToSHA512 := make(map[plumbing.Hash][64]byte)

	sha1ToBranch := make(map[plumbing.Hash]string)
	branchToSHA1 := make(map[string]plumbing.Hash)

	sha512ToBranch := make(map[[64]byte]string)

	for _, after := range repo.After {
		var sha1 plumbing.Hash
		if after.SHA1 != nil {
			sha1 = plumbing.NewHash(*after.SHA1)
			afterSHA1.Add(sha1)

			if after.Branch != nil {
				sha1ToBranch[sha1] = *after.Branch
				branchToSHA1[*after.Branch] = sha1
			}
		}

		var sha512 [64]byte
		if after.SHA512 != nil {
			h, err := hex.DecodeString(*after.SHA512)
			if err != nil {
				return nil, err
			}

			if len(h) != 64 {
				return nil, fmt.Errorf("SHA-512 hash should be 64 bytes, got %d", len(h))
			}

			copy(sha512[:], h[:])

			afterSHA512.Add(sha512)

			if after.Branch != nil {
				sha512ToBranch[sha512] = *after.Branch
			}
		}

		if after.SHA1 != nil && after.SHA512 != nil {
			afterSHA1ToSHA512[sha1] = sha512
		}
	}

	return &RepoConfig{
		afterSHA1:                                  afterSHA1,
		afterSHA512:                                afterSHA512,
		sha1ToBranch:                               sha1ToBranch,
		branchToSHA1:                               branchToSHA1,
		sha512ToBranch:                             sha512ToBranch,
		afterSHA1ToSHA512:                          afterSHA1ToSHA512,
		maintainerEmails:                           maintainerEmails,
		maintainerOrContributorEmails:              maintainerOrContributor,
		maintainerForgeEmails:                      maintainerForgeEmails,
		maintainerOrContributorForgeEmails:         maintainerOrContributorForgeEmails,
		trustedForge:                               f,
		allowSSHSignatures:                         repo.Rules.AllowSSHSignatures,
		requireSSHUserPresent:                      repo.Rules.RequireSSHUserPresent,
		requireSSHUserVerified:                     repo.Rules.RequireSSHUserVerified,
		allowSSHSHA256:                             repo.Rules.AllowSSHSHA256,
		allowPGPSignatures:                         repo.Rules.AllowPGPSignatures,
		requireSignedTags:                          repo.Rules.RequireSignedTags,
		requireMergeCommits:                        repo.Rules.RequireMergeCommits,
		requireCountersigning:                      repo.Rules.RequireCountersigning,
		requireSHA512:                              repo.Rules.RequireSHA512,
		exemptedTags:                               exemptedTagMap,
		exemptedTagsSHA512:                         exemptedTagSHA512Map,
		protectedBranches:                          repo.ProtectedBranches,
		lockdown:                                   repo.Rules.Lockdown,
		requireDistinctTagIdentities:               repo.Rules.RequireDistinctTagIdentities,
		requireDistinctCountersignTagIdentities:    repo.Rules.RequireDistinctCountersignTagIdentities,
		requireDistinctCountersignCommitIdentities: repo.Rules.RequireDistinctCountersignCommitIdentities,
	}, nil
}

func createSSSHPublicKeyMap(sshPublicKeys []string) (map[string]*ssh.PublicKey, error) {
	keyMap := make(map[string]*ssh.PublicKey)
	for _, sshPublicKey := range sshPublicKeys {
		publicKey, rawKey, err := decodeAndParseSSHPublicKey(sshPublicKey)
		if err != nil {
			return nil, err
		}

		// TODO check for duplicates
		keyMap[string(rawKey)] = &publicKey
	}

	return keyMap, nil
}
