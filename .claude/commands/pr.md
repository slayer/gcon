# Create or Update Pull Request

Create a new PR or update an existing one for the current branch.

## Steps

1. Check if we're on a feature branch (not master/main)
2. Check if there's an existing PR for this branch
3. If PR exists: update the title and description based on commits
4. If no PR exists: create a new PR with:
   - Title derived from branch name or commit messages
   - Description summarizing all commits on the branch
   - Base branch: master
5. Report the PR URL
