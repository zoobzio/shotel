# Contributing to shotel

Thank you for your interest in contributing to shotel! This guide will help you get started.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for all contributors.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/shotel.git`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Run tests: `go test ./...`
6. Commit your changes with a descriptive message
7. Push to your fork: `git push origin feature/your-feature-name`
8. Create a Pull Request

## Development Guidelines

### Code Style

- Follow standard Go conventions
- Run `go fmt` before committing
- Add comments for exported functions and types
- Keep functions small and focused

### Testing

- Write tests for new functionality
- Ensure all tests pass: `go test ./...`
- Include benchmarks for performance-critical code
- Aim for good test coverage

### Documentation

- Update documentation for API changes
- Add examples for new features
- Keep doc comments clear and concise

## Types of Contributions

### Bug Reports

- Use GitHub Issues
- Include minimal reproduction code
- Describe expected vs actual behavior
- Include Go version and OS

### Feature Requests

- Open an issue for discussion first
- Explain the use case
- Consider backwards compatibility

### Code Contributions

#### Core Functionality

New features should:
- Follow existing patterns and conventions
- Include comprehensive tests
- Add documentation with examples
- Maintain separation of concerns between core and providers

#### Custom Transformers

New field transformers should:
- Use `RegisterTransformer()` properly
- Handle type assertions safely
- Include tests for edge cases
- Document behavior clearly

#### Examples

New examples should:
- Demonstrate real-world use cases
- Include tests and benchmarks
- Have a descriptive README
- Follow the existing structure

## Pull Request Process

1. **Keep PRs focused** - One feature/fix per PR
2. **Write descriptive commit messages**
3. **Update tests and documentation**
4. **Ensure CI passes**
5. **Respond to review feedback**

## Testing

Run the full test suite:
```bash
go test ./...
```

Run with race detection:
```bash
go test -race ./...
```

Run benchmarks:
```bash
go test -bench=. ./...
```

## Project Structure

```
shotel/
├── *.go              # Core library files
├── *_test.go         # Tests
├── providers/        # OTEL provider configuration
│   └── providers.go  # Default provider setup
├── .github/          # GitHub workflows and automation
└── docs/             # Documentation
```

## Commit Messages

Follow conventional commits:
- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `test:` Test additions/changes
- `refactor:` Code refactoring
- `perf:` Performance improvements
- `chore:` Maintenance tasks

## Release Process

### Automated Releases

This project uses automated release versioning. To create a release:

1. Go to Actions → Release → Run workflow
2. Leave "Version override" empty for automatic version inference
3. Click "Run workflow"

The system will:
- Automatically determine the next version from conventional commits
- Create a git tag
- Generate release notes via GoReleaser
- Publish the release to GitHub

### Manual Release (Legacy)

You can still create releases manually:
```bash
git tag v1.2.3
git push origin v1.2.3
```

### Known Limitations

- **Protected branches**: The automated release cannot bypass branch protection rules. This is by design for security.
- **Concurrent releases**: Rapid successive releases may fail. Simply retry after a moment.
- **Conventional commits required**: Version inference requires conventional commit format (`feat:`, `fix:`, etc.)

### Commit Conventions for Versioning
- `feat:` new features (minor version: 1.2.0 → 1.3.0)
- `fix:` bug fixes (patch version: 1.2.0 → 1.2.1)  
- `feat!:` breaking changes (major version: 1.2.0 → 2.0.0)
- `docs:`, `test:`, `chore:` no version change

Example: `feat(metrics): add support for exponential histograms`

### Version Preview on Pull Requests
Every PR automatically shows the next version that will be created:
- Check PR comments for "Version Preview" 
- Updates automatically as you add commits
- Helps verify your commits have the intended effect

## Questions?

- Open an issue for questions
- Check existing issues first
- Be patient and respectful

Thank you for contributing to shotel!