# Contributing to OQM

First off, thank you for considering contributing to OQM! 🎉

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When you create a bug report, include as many details as possible:

- **Use a clear and descriptive title**
- **Describe the exact steps to reproduce the problem**
- **Provide specific examples**
- **Describe the behavior you observed and what you expected**
- **Include logs and error messages**
- **Specify your OpenWrt version and architecture**

### Suggesting Enhancements

Enhancement suggestions are welcome! Please provide:

- **A clear and descriptive title**
- **A detailed description of the proposed functionality**
- **Explain why this enhancement would be useful**
- **List any similar features in other projects**

### Pull Requests

1. Fork the repository
2. Create a new branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests (if applicable)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.20 or later
- Make
- nftables (for testing)

### Building

```bash
# Clone repository
git clone https://github.com/abdollahzadehghalejoghi/oqm
cd oqm

# Install dependencies
go mod download

# Build
make build

# Run tests
make test
```

### Code Style

- Follow standard Go formatting (`go fmt`)
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions small and focused

### Project Structure

```
oqm/
├── cmd/oqm/           # Main application entry point
├── internal/          # Internal packages
│   ├── cli/          # CLI commands
│   ├── daemon/       # Background daemon
│   ├── nft/          # nftables integration
│   ├── storage/      # Data persistence
│   ├── notify/       # Notifications
│   └── webui/        # Web UI
├── pkg/              # Public packages
│   ├── config/       # Configuration
│   └── logger/       # Logging
└── openwrt/          # OpenWrt packaging
```

### Testing

Currently, OQM focuses on integration testing. To test:

1. Build the binary
2. Run on an OpenWrt device or VM
3. Test CLI commands
4. Test web UI
5. Test daemon functionality

### Documentation

- Update README.md if adding features
- Add inline comments for complex code
- Update CLI help text if changing commands

## Code Review Process

All submissions require review. We use GitHub pull requests for this purpose.

## Community

- Be respectful and constructive
- Help others when you can
- Share your use cases and experiences

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

## Questions?

Feel free to open an issue with the `question` label!
