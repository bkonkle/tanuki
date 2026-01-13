---
id: TANK-018
title: Dockerfile for Tanuki Agent Container
status: done
priority: high
estimate: M
depends_on: []
workstream: B
phase: 1
---

# Dockerfile for Tanuki Agent Container

## Summary

Create the Dockerfile that builds the container image used for agent environments. This replaces the Nix-based approach with a simpler, faster-to-build Dockerfile.

## Acceptance Criteria

- [x] Ubuntu 24.04 base for broad compatibility
- [x] Claude Code CLI pre-installed
- [x] Common development tools (git, node, python)
- [x] Non-root user for security
- [x] Fast build time (< 5 minutes)
- [x] Multi-platform support (amd64, arm64)
- [x] Pinned versions for reproducibility

## Technical Details

### Dockerfile

```dockerfile
# Dockerfile.tanuki
# Tanuki Agent Container
# Build: docker build -t bkonkle/tanuki:latest -f Dockerfile.tanuki .

FROM ubuntu:24.04

LABEL org.opencontainers.image.title="Tanuki Agent"
LABEL org.opencontainers.image.description="Development container for Claude Code agents"
LABEL org.opencontainers.image.version="1.0.0"

# Avoid prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# System packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Core utilities
    ca-certificates \
    curl \
    wget \
    gnupg \
    sudo \
    # Version control
    git \
    gh \
    # Shell and terminal
    zsh \
    tmux \
    # Editors
    vim \
    neovim \
    # Search tools
    ripgrep \
    fd-find \
    fzf \
    jq \
    # Build essentials
    build-essential \
    # Process tools
    htop \
    procps \
    # Network tools
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

# Node.js 22.x (LTS)
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Python 3 with pip
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
ARG DEV_USER=dev
ARG DEV_UID=1000
ARG DEV_GID=1000

RUN groupadd -g ${DEV_GID} ${DEV_USER} \
    && useradd -m -u ${DEV_UID} -g ${DEV_GID} -s /bin/zsh ${DEV_USER} \
    && echo "${DEV_USER} ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/${DEV_USER} \
    && chmod 0440 /etc/sudoers.d/${DEV_USER}

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Switch to dev user
USER ${DEV_USER}
WORKDIR /home/${DEV_USER}

# Shell configuration
RUN echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc \
    && echo 'alias ll="ls -la"' >> ~/.zshrc \
    && echo 'alias la="ls -A"' >> ~/.zshrc

# fd is installed as fd-find on Ubuntu, create alias
RUN sudo ln -sf /usr/bin/fdfind /usr/local/bin/fd

# Create workspace directory
RUN mkdir -p /workspace

# Working directory for mounted code
WORKDIR /workspace

# Default command - keep container running
CMD ["sleep", "infinity"]
```

### Multi-platform Build

```bash
# Build for both amd64 and arm64
docker buildx create --name tanuki-builder --use
docker buildx build \
    --platform linux/amd64,linux/arm64 \
    -t bkonkle/tanuki:latest \
    -f Dockerfile.tanuki \
    --push \
    .
```

### Local Build Script

```bash
#!/bin/bash
# scripts/build-image.sh

set -e

IMAGE_NAME="${IMAGE_NAME:-bkonkle/tanuki}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

echo "Building ${IMAGE_NAME}:${IMAGE_TAG}..."

docker build \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -f Dockerfile.tanuki \
    .

echo "Done! Run with:"
echo "  docker run -it ${IMAGE_NAME}:${IMAGE_TAG}"
```

### Version Pinning Strategy

For reproducibility, consider pinning major versions:

```dockerfile
# Option: Pin Node.js version
ARG NODE_VERSION=22
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash -

# Option: Pin Claude Code version
ARG CLAUDE_CODE_VERSION=latest
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}
```

### Image Size Optimization

Expected image size: ~800MB - 1GB

Optimization strategies already applied:
- `--no-install-recommends` for apt packages
- Clean up apt cache after installs
- Single RUN commands where possible

### Testing

```bash
# Test the image
docker run --rm -it bkonkle/tanuki:latest zsh -c "
    echo 'Testing tools...'
    git --version
    node --version
    python3 --version
    claude --version
    rg --version
    fd --version
    echo 'All tools available!'
"
```

## File Location

```
tanuki/
├── Dockerfile.tanuki    # Main Dockerfile
├── scripts/
│   └── build-image.sh   # Build helper script
└── ...
```

## Out of Scope

- Nix-based image (replaced by this)
- SSH server (agents use docker exec)
- Tailscale (handled separately in compose)

## Notes

This replaces the complex Nix flake with a straightforward Dockerfile that builds quickly on any platform. The trade-off is slightly less reproducibility, but much better developer experience.
