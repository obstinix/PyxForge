# Contributing to PyxForge

Thank you for contributing to PyxForge! Please follow these conventions for all work:

## Branching Model
- Create and work on feature branches named `pyxforge/<description>` (e.g., `pyxforge/phase-0-core-setup`).
- Avoid committing directly to `main`.

## Git Commit Policy
- Commit granularly, on every meaningful unit of work (e.g., single file or cohesive tool scaffolding).
- Push after every commit. Do not squash or force-push.

## Commit Messages
We enforce the Conventional Commits specification. Messages should follow the format:
```
type(scope): short description
```

Valid types include:
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes
- `chore`: Maintenance tasks, dependencies, scaffolding
- `refactor`: Code restructuring without functional changes
- `test`: Adding or correcting tests
- `build`: Build system or toolchain changes
- `ci`: CI configuration changes

Valid scopes include `core` or `extension` (omit for repo-wide changes).
