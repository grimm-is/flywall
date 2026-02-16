---
trigger: always_on
---

# Commit Message Format

Subject format: <type>(<scope>): <subject>

Rules:
1. <type> must be one of: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert.
2. <scope> is optional. Derive it from the primary directory or component modified (e.g. 'lib/auth' -> 'auth'). If multiple components are changed or it's a general change, omit the scope.
3. <subject> must be lowercase, imperative mood ("fix" not "fixed"), and no trailing period.
4. Limit the subject and body lines to 72 characters.
5. Establish the intent of the change in the message body. Prefer lists over paragraphs for visual clarity. Call out specific changes to tools and modules. Be terse but not obtuse.
6. Output ONLY the raw commit message. No markdown code blocks.
