# Instructions for AI agents

## Code Quality

Run `make fix` after every code change.

## Formatting

When writing Markdown, do not hard-wrap text at 80, 100, or 120 characters. Write each paragraph on a single line. Readers can use soft wrapping to read it.

## Tests

Do not add tests unless explicitly asked.

## Commits

Do not create commits unless explicitly asked.

Before creating a commit, run `make fix` to ensure the code follows the linter rules.

If all or most of a commit was created with AI, include a `Co-authored-by` trailer.

Avoid the Conventional Commits format. Just describe what changed.

## Documentation

All user-facing documentation is written in Russian.

Keep `AGENTS.md` up-to-date. This file should be in English. Maintain this file when changes to the project structure, tooling, architecture, or development workflow make its guidance outdated.

The parent directory may contain a directory with the same name as this repository and a `.wiki` suffix, e.g. `anime365-sidecar.wiki`. This is the GitHub Wiki repository. Keep it up-to-date too. Do not commit there unless explicitly asked. If you update the wiki repository, remind the developer to push those changes.

Treat [.env.example](./.env.example) and the environment variables in the Docker Compose configuration in [README.md](./README.md) as the minimal set of variables required to run the app. Do not list optional variables or variables with default values there. Document all optional variables in the wiki repository.
