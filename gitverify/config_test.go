package gitverify

import (
	"encoding/json"
	"testing"
)

func TestConfig(t *testing.T) {
	config := `
{
  "_type": "https://supply-chain-tools.github.io/schemas/gitverify/v0.3",
  "identities": [
    {
      "email": "a@example.internal",
      "sshPublicKeys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIAQv90+kSOSKZYlMoWO0eX6QZ1Nt5n2BviA4vFx3lgK"]
    },
    {
      "email": "b@example.internal",
      "sshPublicKeys": ["ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBH2r8kV3iq50ugWjL3l4OaLEhGNUhMPc/A2UWQSix/I5XEG6sfnXZre06ROUF2DaWxiACUiLhO1UDUY0guun3ZQ="],
      "forgeUsername" : "b",
      "forgeUserId" : "1234"
    },
    {
      "email": "c@example.internal",
      "sshPublicKeys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIAQv90+kSOSKZYlMoWO0eX6QZ1Nt5n2BviA4vFx3lgK"]
    },
    {
      "email": "d@example.internal",
      "sshPublicKeys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIAQv90+kSOSKZYlMoWO0eX6QZ1Nt5n2BviA4vFx3lgK"]
    }
  ],
  "forgeIdentity": {
    "email": "noreply@github.com",
    "gpgPublicKeys": [
      "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n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\n=uMz0\n-----END PGP PUBLIC KEY BLOCK-----\n"
    ]
  },
  "maintainers": ["a@example.internal"],
  "contributors": ["c@example.internal"],
  "rules": {
    "allowGPGSignatures": true,
    "allowSSHSignatures": true,
    "requireSSHUserPresent": true,
    "requireSSHUserVerified": true,
	"trustForge": true
  },
  "trustedForge": "github.com",
  "protectedBranches": ["main"],
  "repositories": [
    {
      "uri": "git+https://github.com/foo/bar.git",
      "after": [{
          "SHA1": "0000000000000000000000000000000000000000"
      }]
    },
    {
      "uri": "git+ssh://github.com/foo/baz.git",
      "after": [{
          "SHA1": "ffffffffffffffffffffffffffffffffffffffff"
      }],
      "identities": [
        {
          "email": "b@example.internal",
          "sshPublicKeys": ["ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBH2r8kV3iq50ugWjL3l4OaLEhGNUhMPc/A2UWQSix/I5XEG6sfnXZre06ROUF2DaWxiACUiLhO1UDUY0guun3ZQ="],
          "forgeUsername" : "b",
          "forgeUserId" : "1234"
        },
        {
          "email": "d@example.internal",
          "sshPublicKeys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIAQv90+kSOSKZYlMoWO0eX6QZ1Nt5n2BviA4vFx3lgK"]
        }
      ],
      "maintainers": ["b@example.internal"],
      "contributors": ["d@example.internal"],
      "rules": {
        "allowSSHSignatures": true,
        "requireSSHUserPresent": false,
        "requireSSHUserVerified": false,
        "trustForge": false
      },
      "protectedBranches": ["release"]
    }
  ]
}
`
	runnerConfig := &Config{}
	err := json.Unmarshal([]byte(config), runnerConfig)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := parseConfig(runnerConfig)
	if err != nil {
		t.Fatal(err)
	}

	repo0 := parsed.Repositories[0]
	if repo0.Uri != "git+https://github.com/foo/bar.git" {
		t.Errorf("repo0.Uri=%q, want %q", repo0.Uri, "git+https://github.com/foo/bar.git")
	}

	if repo0.TrustedForge.Email != "noreply@github.com" {
		t.Errorf("forgeIdentity.email=%q, want %q", repo0.TrustedForge.Email, "github.com")
	}

	if *repo0.After[0].SHA1 != "0000000000000000000000000000000000000000" {
		t.Errorf("repo0.Since[0].SHA1=%v, want %q", *repo0.After[0].SHA1, "0000000000000000000000000000000000000000")
	}

	if repo0.Identities[0].Email != "a@example.internal" {
		t.Errorf("repo0.Identities[0].Email=%q, want %q", repo0.Identities[0].Email, "a@example.internal")
	}

	if repo0.Identities[0].SSHPublicKeys[0] != "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIAQv90+kSOSKZYlMoWO0eX6QZ1Nt5n2BviA4vFx3lgK" {
		t.Errorf("repo0.Identities[0].SSHPublicKeys[0]=%q, want %q", repo0.Identities[0].SSHPublicKeys[0], "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIAQv90+kSOSKZYlMoWO0eX6QZ1Nt5n2BviA4vFx3lgK")
	}

	if !repo0.Maintainers.Contains("a@example.internal") {
		t.Errorf("repo0.Maintainers does not contain %q", "a@example.internal")
	}

	if !repo0.Contributors.Contains("c@example.internal") {
		t.Errorf("repo0.Contributors does not contain %q", "c@example.internal")
	}

	if repo0.Rules.AllowSSHSignatures != true {
		t.Errorf("repo0.Rules.AllowSSHSignature=%t, want %t", repo0.Rules.AllowSSHSignatures, true)
	}

	if repo0.Rules.RequireSSHUserPresent != true {
		t.Errorf("repo0.Rules.RequireSSHUserPresent=%t, want %t", repo0.Rules.RequireSSHUserPresent, true)
	}

	if repo0.Rules.RequireSSHUserVerified != true {
		t.Errorf("repo0.Rules.RequireSSHUserVerified=%t, want %t", repo0.Rules.RequireSSHUserVerified, true)
	}

	if repo0.Rules.AllowGPGSignatures != true {
		t.Errorf("repo0.Rules.AllowGPGSignatures=%t, want %t", repo0.Rules.AllowGPGSignatures, true)
	}

	if repo0.Rules.TrustForge != true {
		t.Errorf("repo0.Rules.TrustForge=%t, want %t", repo0.Rules.TrustForge, true)
	}

	if repo0.ProtectedBranches.Size() != 1 {
		t.Errorf("expected exactly one protected branch, got %d", repo0.ProtectedBranches.Size())
	}

	if !repo0.ProtectedBranches.Contains("main") {
		t.Errorf("expected exactly protected branch to be present")
	}

	repo1 := parsed.Repositories[1]
	if repo1.Uri != "git+ssh://github.com/foo/baz.git" {
		t.Errorf("repo1.Uri=%q, want %q", repo1.Uri, "git+ssh://github.com/foo/baz.git")
	}

	if *repo1.After[0].SHA1 != "ffffffffffffffffffffffffffffffffffffffff" {
		t.Errorf("repo1.Since[0].SHA1=%v, want %q", *repo1.After[0].SHA1, "ffffffffffffffffffffffffffffffffffffffff")
	}

	if repo1.Identities[1].Email != "b@example.internal" {
		t.Errorf("repo1.Identities[0].Email=%q, want %q", repo1.Identities[1].Email, "a@example.internal")
	}

	if repo1.Identities[1].SSHPublicKeys[0] != "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBH2r8kV3iq50ugWjL3l4OaLEhGNUhMPc/A2UWQSix/I5XEG6sfnXZre06ROUF2DaWxiACUiLhO1UDUY0guun3ZQ=" {
		t.Errorf("repo1.Identities[0].SSHPublicKeys[0]=%q, want %q", repo1.Identities[1].SSHPublicKeys[0], "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBH2r8kV3iq50ugWjL3l4OaLEhGNUhMPc/A2UWQSix/I5XEG6sfnXZre06ROUF2DaWxiACUiLhO1UDUY0guun3ZQ=")
	}

	if *repo1.Identities[1].ForgeUsername != "b" {
		t.Errorf("repo1.Identities[0].ForgeUsername=%q, want %q", *repo1.Identities[1].ForgeUsername, "b")
	}

	if *repo1.Identities[1].ForgeUserId != "1234" {
		t.Errorf("repo1.Identities[0].ForgeUserId=%q, want %q", *repo1.Identities[1].ForgeUserId, "1234")
	}

	if !repo1.Maintainers.Contains("b@example.internal") {
		t.Errorf("repo1.Maintainers does not contain %q", "b@example.internal")
	}

	if !repo1.Contributors.Contains("d@example.internal") {
		t.Errorf("repo1.Contributors[0] does not contain %q", "d@example.internal")
	}

	if repo1.Rules.AllowSSHSignatures != true {
		t.Errorf("repo1.Rules.AllowSSHSignature=%t, want %t", repo1.Rules.AllowSSHSignatures, true)
	}

	if repo1.Rules.RequireSSHUserPresent != false {
		t.Errorf("repo1.Rules.RequireSSHUserPresent=%t, want %t", repo1.Rules.RequireSSHUserPresent, false)
	}

	if repo1.Rules.RequireSSHUserVerified != false {
		t.Errorf("repo1.Rules.RequireSSHUserVerified=%t, want %t", repo1.Rules.RequireSSHUserVerified, false)
	}

	if repo1.Rules.AllowGPGSignatures != true {
		t.Errorf("repo1.Rules.AllowGPGSignatures=%t, want %t", repo1.Rules.AllowGPGSignatures, true)
	}

	if repo1.Rules.TrustForge != false {
		t.Errorf("repo0.Rules.TrustForge=%t, want %t", repo1.Rules.TrustForge, false)
	}

	if repo1.ProtectedBranches.Size() != 2 {
		t.Errorf("expected exactly one protected branch, got %d", repo0.ProtectedBranches.Size())
	}

	if !repo1.ProtectedBranches.Contains("main") {
		t.Errorf("expected exactly protected branch to be present")
	}

	if !repo1.ProtectedBranches.Contains("release") {
		t.Errorf("expected exactly protected branch to be present")
	}
}
