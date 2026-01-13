# Tanuki Agent Container
# Build: docker build -t bkonkle/tanuki:latest .

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
ARG NODE_VERSION=22
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash - \
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

# Remove existing ubuntu user if present, then create our dev user
RUN userdel -r ubuntu 2>/dev/null || true \
    && groupdel ubuntu 2>/dev/null || true \
    && groupadd -g ${DEV_GID} ${DEV_USER} 2>/dev/null || true \
    && useradd -m -u ${DEV_UID} -g ${DEV_GID} -s /bin/zsh ${DEV_USER} \
    && echo "${DEV_USER} ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/${DEV_USER} \
    && chmod 0440 /etc/sudoers.d/${DEV_USER}

# Install Claude Code globally
ARG CLAUDE_CODE_VERSION=latest
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}

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
RUN sudo mkdir -p /workspace && sudo chown ${DEV_USER}:${DEV_USER} /workspace

# Working directory for mounted code
WORKDIR /workspace

# Default command - keep container running
CMD ["sleep", "infinity"]
