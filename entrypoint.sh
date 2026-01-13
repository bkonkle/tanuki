#!/bin/bash
set -e

DEV_USER="${DEV_USER:-brandon}"
DEV_HOME="/home/${DEV_USER}"
SSH_DIR="${DEV_HOME}/.ssh"
PRIVATE_KEY="${SSH_DIR}/id_ed25519"
PUBLIC_KEY="${SSH_DIR}/id_ed25519.pub"
CLAUDE_CONFIG_DIR="${DEV_HOME}/.config/claude-code"
CLAUDE_AUTH_FILE="${CLAUDE_CONFIG_DIR}/auth.json"

mkdir -p "${SSH_DIR}"
chown "${DEV_USER}":"${DEV_USER}" "${SSH_DIR}"
chmod 700 "${SSH_DIR}"

# Generate an SSH key pair if one doesn't exist.
if [ ! -f "$PRIVATE_KEY" ]; then
  sudo -u "${DEV_USER}" ssh-keygen -t ed25519 -f "${PRIVATE_KEY}" -N "" -C "${DEV_USER}@tanuki"
  chown "${DEV_USER}":"${DEV_USER}" "${PRIVATE_KEY}" "${PUBLIC_KEY}"
fi

# Set up Claude Code authentication if CLAUDE_CODE_AUTH is provided
if [ -n "${CLAUDE_CODE_AUTH}" ]; then
  echo "Setting up Claude Code authentication..."
  mkdir -p "${CLAUDE_CONFIG_DIR}"
  echo "${CLAUDE_CODE_AUTH}" > "${CLAUDE_AUTH_FILE}"
  chown -R "${DEV_USER}":"${DEV_USER}" "${CLAUDE_CONFIG_DIR}"
  chmod 700 "${CLAUDE_CONFIG_DIR}"
  chmod 600 "${CLAUDE_AUTH_FILE}"
  echo "Claude Code authentication configured successfully"
fi

# Start SSH daemon in the foreground
exec /usr/sbin/sshd -D -e
