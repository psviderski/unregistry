# Contributing to unregistry

Thank you for contributing to unregistry!

## Development Setup

1. **Requirements**
   - Go 1.21+
   - Git

2. **Clone the repository**
   ```bash
   git clone https://github.com/psviderski/unregistry.git
   cd unregistry
   ```

3. **Install dependencies**
   ```bash
   go mod download
   ```

4. **Run tests**
   ```bash
   go test ./...
   ```

5. **Build**
   ```bash
   go build ./...
   ```

## Making Changes

1. **Create a feature branch**
   ```bash
   git checkout -b feat/your-feature-name
   ```

2. **Code style**
   - Follow Go coding conventions (run `go fmt`)
   - Use `go vet` to check for issues
   - Write clear, idiomatic Go code

3. **Commit and push**
   ```bash
   git commit -m "feat: add your feature"
   git push origin feat/your-feature-name
   ```

## Pull Request Process

1. Fork the repository
2. Create your feature branch
3. Make your changes with tests
4. Run `go fmt` and `go vet` before committing
5. Submit a PR with description

## Reporting Issues

- Use GitHub Issues for bugs and feature requests
- Include Go version and OS
- Provide reproduction steps

## License

By contributing, you agree that your contributions will be licensed under the project license.
