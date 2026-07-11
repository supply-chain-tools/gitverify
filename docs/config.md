# Config file

### All config options

```json
{
  "_type": "https://supply-chain-tools.github.io/schemas/gitverify/v0.2",
  "identities": [
    {
      "email": "stian.kristoffersen@telenor.no",
      "forgeUsername": "stiankri-telenor",
      "forgeUserId": "155450741",
      "sshPublicKeys": [
        {
          "key": "sk-ssh-ed25519@openssh.com AAAAGnNrLXNzaC1lZDI1NTE5QG9wZW5zc2guY29tAAAAIHa4MOkvaZbhdeWuYUFQ1sywWYkpW9x9uVTX+RlDdMvXAAAABHNzaDo=",
          "SignCommits": true,
          "SignTags": true,
          "SignCountersignTags": true,
          "SignCountersignCommits": true
        }
      ]
    },
    {
      "email": "33384479+dev-bio@users.noreply.github.com",
      "forgeUsername": "dev-bio",
      "forgeUserId": "33384479",
      "sshPublicKeys": [
        {
          "key": "sk-ssh-ed25519@openssh.com AAAAGnNrLXNzaC1lZDI1NTE5QG9wZW5zc2guY29tAAAAIDTCGpjJM/to9icZbLRiyYzz1UoPDTSbqhwLRotpWd4sAAAABHNzaDo="
        }
      ]
    }
  ],
  "forgeIdentity": {
    "email": "noreply@github.com",
    "pgpPublicKeys": [
      "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nxsBNBFmUaEEBCACzXTDt6ZnyaVtueZASBzgnAmK13q9Urgch+sKYeIhdymjuMQta\nx15OklctmrZtqre5kwPUosG3/B2/ikuPYElcHgGPL4uL5Em6S5C/oozfkYzhwRrT\nSQzvYjsE4I34To4UdE9KA97wrQjGoz2Bx72WDLyWwctD3DKQtYeHXswXXtXwKfjQ\n7Fy4+Bf5IPh76dA8NJ6UtjjLIDlKqdxLW4atHe6xWFaJ+XdLUtsAroZcXBeWDCPa\nbuXCDscJcLJRKZVc62gOZXXtPfoHqvUPp3nuLA4YjH9bphbrMWMf810Wxz9JTd3v\nyWgGqNY0zbBqeZoGv+TuExlRHT8ASGFS9SVDABEBAAHNNUdpdEh1YiAod2ViLWZs\nb3cgY29tbWl0IHNpZ25pbmcpIDxub3JlcGx5QGdpdGh1Yi5jb20+wsBoBBMBCAAc\nBQJZlGhBCRBK7hj4Ov3rIwIbAwUJDBJ3/wIZAQAA0O4IAJd0k8M+urETyMvTqNTj\n/U6nbqyOdKE4V93uUj5G7sNTfno7wod/Qjj6Zv5KodvA93HmEdQqsmVq5YJ5KGiw\ncmGCpd/GqJRPaYSY0hSUSBqYHiHLusCJkPBpQTBhcEMtfVCB2J6fVeoX2DV0K1xf\nCGblrSVB0viAxUMnmL5C55RuvbYZsTu8szXhkvIR96CtWbJ8QGaEf1/KSpWz8ept\nY/omf3UPfvdOjnsxc8jVEqPNaR9xC6Q6t53rBa/XgMY6IYyesnyYnc5O6JuexUFa\nVjykRFtAiYfDaMARpXOmgMm0lhoBRKb/uMUaN3CSYTmE4pZweJcUi7eWgmoQljX2\nut6ZAg0EZabFdgEQALI37i+IVAzpBCgqvQDZbSsZ0yhtMnA5myjZA+l7BvIGy4ve\ns1bk6YetbBcCE8o2pQjI7N2rwyhLGhNO6ouSyhqGLEQv9fafKE4HFH0aRjP+gj1H\nedhwtFoVChImhV863rWimQtTNtYB6GluBPwQqWfwmwQ2rT7ScOVZCLSHZD2gaaqW\nBXOyTCZVnwt7K/gyDuE3qzDJnuahl+SSkPn5TtnZdW6sLORJJ+DjNvaUxEsmizZ4\nIBzvj0QKxfS3s4F+0X5iqCMheLFeybZGtSq9Tjs6Q61l4CG8Bh6dsLemv0WFrk3G\ngFQRr7XUwr1bo5xGHC/FUJSsxRHoVNJnIL/9WldNO2tGU6qlTnAYxs/fOmf2B6o5\ncKXysXv7WAA8b+j5AVBMGxUSu7CLglaiCJC5DI7AAiUV7/t29rFZkam//Jbb4veC\n4vvFocoVUaxrKGWK1BDldr4/WJKApJcPJF4Jtai1+oB6ak/JIjbkseHdJxcjo2B0\ndKtIFoWiPAB+DFs9MRDpp0iwocJCh+ucus1rdQ54YMaI44rRphXeOIQMYCi5q2Q1\n/arzkSiyPV/2VoKoAfdgskPt1xKd7WIKErmpFMHIy8jJ5IPQ1s2dUwU4alfJLJa0\npvaV2m7wBYFAmwmz0WZgFxYAYEDamn4jFoKfqsEgcixRUVE3w5VkqwSwGRbLABEB\nAAG0G0dpdEh1YiA8bm9yZXBseUBnaXRodWIuY29tPokCTgQTAQoAOBYhBJaEeaGv\n+SfjfRpWa7VpDu67lSGUBQJlpsV2AhsDBQsJCAcCBhUKCQgLAgQWAgMBAh4BAheA\nAAoJELVpDu67lSGUgy4QAKW9XAL416iKrQB2LElmxqAoenHVCswlau0xGLh5dVNN\np5f4/W6eEL8CZI7hfF3e5Gh6Me99aHgXSCK1QnxcqCJ6Oea4ZyrsNu3k6g7Um5ca\nVbYFD4yIahhXDYHSw6FYM2sgFY479YvgvKRwacC2tFfChLRbHgwLJ3O1dBjmVycJ\nZpbyu+7taZ26g6KQfgcj3uuo3nz3p1ziIEpLHwtl/7joNEIIP/lJ8AKmUHPiGznN\n6fxMvzN37PGMWtdvOi1rSNIMQYr1YY7jPnlLbFJwLrO/q/cGPU5HwGzlqh0a2ZqY\ndnuwT3DREmgJ83H71xH+sTzZKs5oGlVTu6st7iWDvNpo2GoN01XzKa5caYglqsOC\nuZ6IHlsdL50sXMtSROCi3hEWU9r1sWIm4k3pNz20y7lElD2X/MqbEMcgpawCV7lH\nrm7MSrTgu6BNAF0SisbF9AKwXaBr2dwpMMyIBOFZO9mk4/c0n9q2FlGY4GkbgH2J\nHqulFTwX/4yiQbh8gzCe+06FZAWITN1OQntTkkCQ+1MCZPf+bOfC08RTsOsVZIYB\n2qAgw6XE0IF4a+PAtHSoYftwH2ocMY2gMuSNpQWm7m0+/j+K+RBoeUcnGNPQgszq\nN60IDMqkqHjyubrm2aslfopWmPSvaQoyxwV/uztdo+UI0IV2z9gD7Sm49vMkpYp8\n=uMz0\n-----END PGP PUBLIC KEY BLOCK-----\n"
    ]
  },
  "maintainers": [
    "stian.kristoffersen@telenor.no"
  ],
  "contributors": [
    "33384479+dev-bio@users.noreply.github.com"
  ],
  "rules": {
    "allowSshSignatures": true,
    "requireSshUserPresent": false,
    "requireSshUserVerified": false,
    "allowPgpSignatures": true,
    "requireSignedTags": true,
    "requireMergeCommits": true,
    "requireCountersigning": false,
    "requireSha512": false,
    "trustForge": false
  },
  "protectedBranches": [
    "main"
  ],
  "trustedForge": null,
  "repositories": [
    {
      "uri": " git+https://github.com/supply-chain-tools/gitverify.git",
      "after": [
        {
          "sha1": "88fc58debf5fc1e36c2e6ecf94447a084eb7aeee",
          "branch": "main"
        }
      ],
      "exemptTags": [
        {
          "ref": "refs/tags/0.0.1",
          "hash": {
            "sha1": "1f46f2053221c040ce5bcba0239bc09214a37658"
          }
        }
      ]
    }
  ]
}
```

### Identities
| Config                               | Value                          | Required | Description                                                            |
|--------------------------------------|--------------------------------|----------|------------------------------------------------------------------------|
| `identities`                         | list of `identity`             | yes      |                                                                        |
| `identity.email`                     | email                          | yes      | Must be unique for a `repository`                                      |
| `identity.sshPublicKeys`             | list of `sshPublicKey` objects | no       | See description below.                                                 |
| `identity.pgpPublicKeys`             | list of `pgpPublicKey` objects | no       | See description below.                                                 |
| `identity.forgeUsername`             | string                         | no       | E.g. GitHub login name                                                 |
| `identity.forgeUserId`               | string                         | no       | E.g. GitHub user id                                                    |

### SSH and PGP keys
| Config                                      | Value                     | Required   | Description                                                                                                |
|---------------------------------------------|---------------------------|------------|------------------------------------------------------------------------------------------------------------|
| `sshPublicKey`                              | object                    | no         |                                                                                                            |
| `pgpPublicKey`                              | object                    | no         |                                                                                                            |
| `sshPublicKey.key`                          | SSH key                   | yes        | The same format as in SSH public files without the comment.                                                |
| `pgpPublicKey.key`                          | GPG key                   | yes        | Only one PGP key per pupose is currently supported, standard armored string with newlines encoded as `\n`. |
| `{ssh,pgp}PublicKey.signCommits`            | `true` (default), `false` | no         | Use key to sign all commits except countersign commits.                                                    |
| `{ssh,pgp}PublicKey.signTags`               | `true` (default), `false` | no         | Use key to sign all tags except countersign tags.                                                          |
| `{ssh,pgp}PublicKey.signCountersignTags`    | `true` (default), `false` | no         | Use key to sign countersign tags.                                                                          |
| `{ssh,pgp}PublicKey.signCountersignCommits` | `true` (default), `false` | no         | Use key to sign countersign commits.                                                                       |

### Forge Identity
While gitverify is primarily intended to verify local signatures, it support being able to trust the signatures of a forge.
To enable it set `forgeIdentity` and `rules.trustForge`.

| Config                   | Value                   | Required | Description                             |
|--------------------------|-------------------------|----------|-----------------------------------------|
| `forgeIdentity`          | object                  | no       |                                         |
| `identity.email`         | email                   | yes      | Only 'noreply@github.com' is supported. |
| `identity.pgpPublicKeys` | list of PGP public keys | yes      | Only one PGP key is supported.          |

### Maintainers and Contributors
Maintainers are allowed to sign any commit or tag. Contributors are not allowed to sign tags. Merge commits into
`protectedBranches` will be verified to be from maintainers, not contributors.
If `trustForge` is set, then the author is verified to match `maintainers`
or `contributors` following the same rules as if they made the commit themselves.

The difference between `maintainers` and `contributors` might change in the future. The main goal is to allow for outside
contributions without a maintainer committing the change.

| Config        | Value              | Required | Description                        |
|---------------|--------------------|----------|------------------------------------|
| `maintainers` | list of emails     | yes      | Must reference an `identity.email` |

| Config         | Value              | Required | Description                        |
|----------------|--------------------|----------|------------------------------------|
| `contributors` | list of emails     | no       | Must reference an `identity.email` |

### Rules
| Config                                        | Value                     | Required | Description                                                                                                                                                                                                                                                                                                                                                                                                                 |
|-----------------------------------------------|---------------------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `rules`                                       | object                    | no       |                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `rules.allowSshSignatures`                    | `true` (default), `false` | no       | `maintainers` and `contributors` are allowed to use SSH signatures                                                                                                                                                                                                                                                                                                                                                          |
| `rules.requireSshUserPresent`                 | `true`, `false` (default) | no       | `maintainers` and `contributors` are required to touch security key when signing. Only `sk-ssh-ed25519@openssh.com` and `sk-ecdsa-sha2-nistp256@openssh.com` is supported and will fail on other key types.                                                                                                                                                                                                                 |
| `rules.requireSshUserVerified`                | `true`, `false` (default) | no       | `maintainers` and `contributors` are required to use PIN with security key when signing. Only `sk-ssh-ed25519@openssh.com` and `sk-ecdsa-sha2-nistp256@openssh.com` is supported and will fail on other key types.                                                                                                                                                                                                          |
| `rules.allowSshSha256`                        | `true`, `false` (default) | no       | Allow SHA-256 to be used in the SSH signature.                                                                                                                                                                                                                                                                                                                                                                              |
| `rules.allowPgpSignatures`                    | `true` (default), `false` | no       | `maintainers` and `contributors` are allowed to use PGP signatures                                                                                                                                                                                                                                                                                                                                                          |
| `rules.requireSignedTags`                     | `true` (default), `false` | no       | Allow unsigned tags, `repository.exemptTags` is an alternative                                                                                                                                                                                                                                                                                                                                                              |
| `rules.requireMergeCommits`                   | `true` (default), `false` | no       | Require protected branches to use merge commits. Any conflicts must be resolved before merging.                                                                                                                                                                                                                                                                                                                             |
| `rules.requireCountersigning`                 | `true`, `false` (default) | no       | Require protected branches to use countersinging via mergetags. The committer and tagger must be different identities. The tree of the merge commit must be the same as the tree in the tagged commit.                                                                                                                                                                                                                      |
| `rules.requireSha512`                         | `true`, `false` (default) | no       | Require SHA-512 in countersigned commits (via `Gitverify-object-sha512: <hex>` in the mergetag, which can be created using the [pr CLI](pr.md)). SHA-512 is also required in other places like `after.sha512`, `exemptTag.hash.sha512`. **For SSH, depending on the signature algorithm, SHA-256, SHA-384, or SHA-512 is used.** PGP is currently not supported. Regular tags and commits are not affected by this setting. |
| `rules.lockdown`                              | `true`, `false` (default) | no       | All commits in the repository must have a valid signature from a maintainer or a contributor. All refs must be connected to a `repository.after`.                                                                                                                                                                                                                                                                           |
| `rules.trustForge`                            | `true`, `false` (default) | no       | Trust signatures made by `forgeIdentity`                                                                                                                                                                                                                                                                                                                                                                                    |
| `rules.requireDedicatedTagKeys`               | `true`, `false` (default) | no       | `{ssh,pgp}PublicKeys` used for tags cannot have other purposes.                                                                                                                                                                                                                                                                                                                                                             |
| `rules.requireDedicatedCountersignTagKeys`    | `true`, `false` (default) | no       | `{ssh,pgp}PublicKeys` used for countersign tags cannot have other purposes.                                                                                                                                                                                                                                                                                                                                                 |
| `rules.requireDedicatedCountersignCommitKeys` | `true`, `false` (default) | no       | `{ssh,pgp}PublicKeys` used for countersign commits cannot have other purposes.                                                                                                                                                                                                                                                                                                                                              |


### Protected branches
Only maintainers can commit on a protected branch.
When `requireMergeCommits` is set, only merge commits are allowed into the protected branch (no rebase/squash/plain commit).
When `lockdown` is set all commits must be have valid signatures, otherwise commits being merged in are not checked.

| Config              | Value                | Required | Description  |
|---------------------|----------------------|----------|--------------|
| `protectedBranches` | list of branch names | no       | E.g. `main`  |

### Repository
| Config                  | Value                | Required                                   | Description                                                                                                                                         |
|-------------------------|----------------------|--------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------|
| `repositories`          | list of `repository` | yes                                        | Used to verify forge commits and interpret `identities.forgeUsername` and `identities.forgeUserId`                                                  |
| `repository.uri`        | repo URI             | yes                                        | E.g. `git+https://github.com/supply-chain-tools/gitverify.git`                                                                                      |
| `repository.after`      | list of `after`      | yes                                        |                                                                                                                                                     |
| `after.sha1`            | git commit SHA-1     | yes, unless `after.sha512` is set          | The commit pointed to by `after.sha1` and it's ancestors will be ignored. If both `sha1` and `sha512` are set they must point to the same commit.   |
| `after.sha512`          | git commit SHA-512   | yes, unless `after.sha1` is set            | The commit pointed to by `after.sha512` and it's ancestors will be ignored. If both `sha1` and `sha512` are set they must point to the same commit. |
| `after.branch`          | branch name          | no, unless `protectedBranches` are used    | Associate the `after` hashes with a branch. This is used to verify protected branches.                                                              |
| `repository.exemptTags` | list of `exemptTag`  | no                                         | List of tags that will not be verified                                                                                                              |
| `exemptTag.ref`         | name of tag          | yes                                        | E.g. `refs/tags/v0.0.1`                                                                                                                             |
| `exemptTag.hash`        | object               | yes                                        | All hashes must point to the same tag                                                                                                               |
| `exemptTag.hash.sha1`   | git SHA-1            | yes, unless `exemptTag.hash.sha512` is set | Contents of `repository.exemptTags.ref`: hash of an annotated tag or a commit (for lightweight tags)                                                |
| `exemptTag.hash.sha512` | git SHA-512          | yes, unless `exemptTag.hash.sha1` is set   | Contents of `repository.exemptTags.ref`: hash of an annotated tag or a commit (for lightweight tags)                                                |

Generate `repositories.exemptTags`:
```sh
$ gitverify exempt-tags
[{"ref":"refs/tags/0.0.1","hash":{"sha1":"1f46f2053221c040ce5bcba0239bc09214a37658"}}]
```

Generate candidates for `after`
```sh
$ gitverify after-candidates
```

| Per repository changes to global config | Value               | Required | Description                                   |
|-----------------------------------------|---------------------|----------|-----------------------------------------------|
| `repository.maintainers`                | `maintainers`       | no       | Merge with global `maintainers` section       |
| `repository.contributors`               | `contributors`      | no       | Merge with global `contributors` section      |
| `repository.rules.*`                    | `rules.*`           | no       | Override individual global `rules.*` values   |
| `repository.protectedBranches`          | `protectedBranches` | no       | Merge with global `protectedBranches` section |

