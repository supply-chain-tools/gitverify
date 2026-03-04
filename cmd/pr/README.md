# pr

A helper tool to be used with the countersigning feature in gitverify.

## Example

### Creator
Create PR tag, this assumes PR #1 already exists and is not expected to receive further changes
```
pr tag 1
```

Push tag
```
git push origin pr/1
```

### Countersigner
Reviews `pr/1`, then merges locally
```
pr merge 1
```

This will print the GitHub link to approve the same commit in the PR.

After the PR is approved, it can then be pushed
```
git push origin main
```

## Format

### Create PR tag
Create a PR tag by tagging a commit with the message
```
PR https://github.com/supply-chain-tools/gitverify/pull/1

optional message

Object-sha512: <SHA-512 of the commit the tag points to>
```

### Merge commit
Merge commit with a tag, with the message
```
Merged PR https://github.com/supply-chain-tools/gitverify/pull/1

optional message
```
