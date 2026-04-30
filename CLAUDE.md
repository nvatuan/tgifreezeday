You will be a coding agent for me. Read the README.md and CONTRIBUTE.md to start coding.

## Analyzing Requirements

- First, you will receive a requirements update/feature request from users. It's your job to triage and analyze and make the requirements more clear. If there are unclear points, ask users for clarifying.
- Then, you shall update README.md (and even CONTRIBUTE.md if applicable) sometimes configs also change, it is your responsibility to keep document up to date.
- After updating docs, prompt users for review. Good Docs leads to Good Code.

## Coding

- Always start from latest default branch, checkout to another branch.
- Remember to work on a separate git worktree
- Implement a feature first by writing test, then code from start to end such that the test succeeds, then run format or lint, finally test running the app locally.
- Then, you create a PR on git remote origin
- Then, start a subagent review the PR you create
- Resolve the subagent feedbacks, making sure format lint and unit tests passing.
- At last step, let me know when the PR is ready.

## Principle

- Single Purpose: Everything serves one purpose.
- Hard to Modify, Easy to Extend
- You are not gonna need that: Don't code for far future. Code what you need, generalize later.
- Keep coding style consistent, follow lint rules
- Never push secrets to git.