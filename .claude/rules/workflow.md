---
description: Development workflow, git conventions, testing practices
---

# Development Workflow

## Task Management

- Use current date in format `YYYY-mm-dd` for `{task_id}`, like `2025-12-11`
- Before starting any task, check if similar task is already in progress or completed
- For every new task, create a new branch named `{task_id}-<short_description>`
- On every task, create a `doc/{task_id}-{short description}/TODO.md` file
  - For subtasks, create a `doc/{task_id}-{short description}/TODO_<subtask_name>.md` file
- Use this file to outline the task:
  - Task description
  - Implementation plan
  - Any specific requirements or constraints
- Break down the task into smaller, manageable steps
- Use TODO file to track progress and document decisions
- Periodically mark completed tasks as done

## Documentation

After completing the task, create/update `doc/{task_id}-{short description}/Documentation.md` with:
- Summary of changes made
- Any relevant technical details
- Instructions for testing or deployment if applicable
- Mermaid diagrams if necessary to illustrate complex workflows

## Code Formatting

Use `go fmt` to format your code before committing.

## Testing

- Run corresponding tests frequently when implementing code edits
- Always run full test suite before deciding a step is done
- Run linter before committing code changes
- Create minimal but comprehensive tests to cover new features or bug fixes

## Branches and Commit Messages

Branch naming: `{task_id}-<short_description>`
- Description should be concise and less than 32 characters
- Example: `2025-12-11-project-selection`

Commit message format:
```
{task_id}: <short description of changes>
```

## Subagents

Spin up multiple subagents for each task to ensure parallel development. Each subagent should work on a specific aspect of the task, such as implementation, testing, or documentation.

## Git and Pull Requests

- Use Git for version control
- Project hosted on GitHub
- Create Pull Requests for code reviews before merging to main
- Use descriptive titles and descriptions for PRs
- Use GitHub MCP if available for GitHub related tasks
