# Contributing to shotel

Thank you for your interest in contributing to shotel! This document provides guidelines and instructions for contributing.

## Code of Conduct

This project adheres to a simple principle: be professional and respectful in all interactions.

## How to Contribute

### Reporting Bugs

Before creating bug reports, please check existing issues to avoid duplicates. When creating a bug report, include:

- A clear and descriptive title
- Steps to reproduce the issue
- Expected behavior vs actual behavior
- Go version and OS information
- Any relevant logs or error messages

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, include:

- A clear and descriptive title
- Detailed description of the proposed functionality
- Explanation of why this enhancement would be useful
- Examples of how the feature would be used

### Pull Requests

1. Fork the repository and create your branch from `main`
2. Make your changes, following the coding standards below
3. Add tests for any new functionality
4. Ensure all tests pass: `make test`
5. Ensure linting passes: `make lint`
6. Ensure code coverage meets requirements: `make coverage`
7. Update documentation as needed
8. Commit using conventional commit format (see below)
9. Push to your fork and submit a pull request

## Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/shotel.git
cd shotel

# Install development tools
make install-tools

# Run tests
make test

# Run linting
make lint

# Generate coverage report
make coverage
```

## Coding Standards

### Go Style

- Follow standard Go conventions and idioms
- Use `gofmt` for formatting (enforced by CI)
- Follow the [Effective Go](https://golang.org/doc/effective_go) guidelines
- Keep functions small and focused
- Write descriptive variable and function names

### Testing

- Write tests for all new functionality
- Maintain or improve code coverage (minimum 70% project, 80% patch)
- Use table-driven tests where appropriate
- Include both positive and negative test cases
- Tests should be deterministic and not depend on timing

### Documentation

- Add GoDoc comments for all exported types, functions, and methods
- Keep comments clear and concise
- Update README.md if adding new features
- Include code examples in documentation where helpful

### Error Handling

- Return errors rather than panicking
- Provide context in error messages
- Use `fmt.Errorf` with `%w` for error wrapping
- Check and handle all errors

## Commit Message Format

This project uses [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Maintenance tasks
- `ci`: CI/CD changes

Examples:
```
feat(metrics): add histogram support

Implements histogram metric collection from metricz.Registry
and translation to OTLP histogram data points.

Closes #42
```

```
fix(traces): handle nil span attributes

Adds nil check before accessing span attributes to prevent
panic when observing traces without metadata.
```

## Release Process

Releases are automated through GitHub Actions:

1. Version tags follow semantic versioning: `vMAJOR.MINOR.PATCH`
2. Tag a commit on `main` to trigger a release
3. GoReleaser creates the GitHub release with changelog
4. Source archives are attached automatically

## Questions?

Feel free to open an issue for any questions about contributing.
