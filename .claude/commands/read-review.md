# Read and Address PR Review Comments

Fetch review comments from the current branch's PR and address any issues.

## Steps

1. Get the current branch name
2. Find the PR associated with this branch
3. Fetch all review comments using: `gh api repos/{owner}/{repo}/pulls/{pr_number}/comments`
4. Parse and display each comment with:
   - File path and line number
   - Comment body
   - Any code suggestions
5. For each comment:
   - Analyze if it's actionable (bug fix, code improvement, etc.)
   - If applicable, implement the fix
   - Run tests to verify the fix doesn't break anything
6. After addressing all comments:
   - Commit the fixes with a descriptive message
   - Push to update the PR

## Output Format

For each comment, show:
```
---
File: {path}:{line}
Issue: {brief description}
Status: {Fixed | Skipped | N/A}
---
```

## Notes

- Skip comments that are just questions or acknowledgments
- Group related comments when possible
- Always run `go build ./...` and `go test ./...` after making changes
