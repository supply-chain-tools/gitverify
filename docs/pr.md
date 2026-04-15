# pr

A helper tool to be used with the countersigning feature in gitverify. It's located in [cmd/pr](../cmd/pr).

## Example

### Creator
Creates the PR tag (this assumes PR #1 already exists and is not expected to receive further changes)
```
pr tag 1
```

Pushes the tag
```
git push origin pr/1
```

### Countersigner
Pulls and reviews `pr/1`, then merges locally
```
pr merge 1
```

This will print the GitHub link to approve the same commit in the PR.

After the PR is approved, it can then be pushed
```
git push origin main
```

## Format

### PR tag message
```
PR https://github.com/supply-chain-tools/gitverify/pull/1

optional message

Gitverify-object-sha512: <hex encoded SHA-512 of the commit the tag points to>
```

### Merge commit message
```
Merged PR https://github.com/supply-chain-tools/gitverify/pull/1

optional message
```
