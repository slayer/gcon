# What I Learned (WIL)

Analyze the current conversation to identify important lessons, patterns, or insights that should be documented in CLAUDE.md.

## Steps

1. Review the conversation history for:
   - Debugging sessions that revealed non-obvious issues
   - Architectural decisions and their rationale
   - Common pitfalls or gotchas discovered
   - Best practices that emerged from problem-solving
   - Framework/library-specific behaviors (e.g., Bubble Tea, lipgloss)
   - Code patterns that worked well or should be avoided

2. For each potential lesson, evaluate:
   - Is it specific to this project or generally applicable?
   - Would a future developer (or AI) benefit from knowing this?
   - Is it already documented in CLAUDE.md?
   - Is it non-obvious enough to warrant documentation?

3. Present findings as a structured list with:
   - Category (e.g., "Bubble Tea Rendering", "Testing", "Architecture")
   - Brief description of the lesson
   - Code example if applicable
   - Recommendation: whether to add to CLAUDE.md

4. If user approves, propose specific additions to CLAUDE.md with exact location and content.

## Output Format

```
## Lessons Identified

### 1. [Category]: [Brief Title]
**Issue**: What problem was encountered
**Root Cause**: Why it happened
**Solution**: How it was fixed
**Recommendation**: Add to CLAUDE.md? [Yes/No] - [reason]

### 2. ...
```
