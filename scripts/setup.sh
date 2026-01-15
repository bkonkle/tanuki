#!/usr/bin/env bash
# Tanuki Development Setup Script
# Run this after cloning the repo to set up your development environment.

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Check for required tools
check_requirements() {
    info "Checking requirements..."

    # Go
    if ! command -v go &> /dev/null; then
        error "Go is not installed. Please install Go 1.21+ from https://go.dev/dl/"
    fi
    GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    info "  Go $GO_VERSION found"

    # Docker
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed. Please install Docker from https://www.docker.com/get-started"
    fi
    info "  Docker found"

    # Git
    if ! command -v git &> /dev/null; then
        error "Git is not installed."
    fi
    info "  Git found"

    # golangci-lint
    if ! command -v golangci-lint &> /dev/null; then
        warn "golangci-lint is not installed."
        info "  Installing golangci-lint..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    else
        info "  golangci-lint found"
    fi
}

# Install pre-commit hooks
setup_precommit() {
    info "Setting up pre-commit hooks..."

    if command -v pre-commit &> /dev/null; then
        info "  pre-commit found, installing hooks..."
        pre-commit install
        info "  Pre-commit hooks installed successfully"
    else
        warn "pre-commit is not installed."
        info "  You can install it via:"
        info "    brew install pre-commit     # macOS"
        info "    pip install pre-commit      # pip"
        info "    pipx install pre-commit     # pipx (recommended)"
        info ""
        info "  Installing fallback git hooks instead..."
        setup_fallback_hooks
    fi
}

# Fallback: Install basic git hooks if pre-commit isn't available
setup_fallback_hooks() {
    HOOKS_DIR=".git/hooks"

    cat > "$HOOKS_DIR/pre-commit" << 'EOF'
#!/usr/bin/env bash
# Fallback pre-commit hook (install pre-commit for full functionality)
set -e

echo "Running pre-commit checks..."

# Format check
echo "  Checking go fmt..."
go fmt ./...
if ! git diff --exit-code --quiet; then
    echo "Error: go fmt made changes. Please stage them and try again."
    exit 1
fi

# Vet
echo "  Running go vet..."
go vet ./...

# Mod tidy check
echo "  Checking go mod tidy..."
go mod tidy
if ! git diff --exit-code --quiet go.mod go.sum; then
    echo "Error: go mod tidy made changes. Please stage them and try again."
    exit 1
fi

# Lint (if available)
if command -v golangci-lint &> /dev/null; then
    echo "  Running golangci-lint..."
    golangci-lint run --config=.golangci.yml ./...
fi

# Tests
echo "  Running tests..."
go test -race ./...

echo "All pre-commit checks passed!"
EOF

    chmod +x "$HOOKS_DIR/pre-commit"
    info "  Fallback pre-commit hook installed"
}

# Download Go dependencies
setup_dependencies() {
    info "Downloading Go dependencies..."
    go mod download
    info "  Dependencies downloaded"
}

# Build the binary
build_binary() {
    info "Building tanuki..."
    make build
    info "  Build successful: bin/tanuki"
}

# Verify the setup
verify_setup() {
    info "Verifying setup..."

    # Run a quick test
    if go test -short ./... > /dev/null 2>&1; then
        info "  Tests pass"
    else
        warn "  Some tests failed (this may be expected if Docker isn't running)"
    fi

    # Check binary runs
    if ./bin/tanuki version > /dev/null 2>&1; then
        info "  Binary runs successfully"
    else
        warn "  Binary verification failed"
    fi
}

main() {
    echo ""
    echo "==================================="
    echo "  Tanuki Development Setup"
    echo "==================================="
    echo ""

    check_requirements
    echo ""
    setup_dependencies
    echo ""
    setup_precommit
    echo ""
    build_binary
    echo ""
    verify_setup

    echo ""
    echo "==================================="
    info "Setup complete!"
    echo "==================================="
    echo ""
    echo "Next steps:"
    echo "  1. Run 'make test' to run the test suite"
    echo "  2. Run 'make lint' to run the linter"
    echo "  3. Run './bin/tanuki --help' to see available commands"
    echo ""
    echo "The pre-commit hooks will now run automatically before each commit."
    echo ""
}

main "$@"
