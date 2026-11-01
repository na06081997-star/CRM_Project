#!/usr/bin/env bash
# Prepares a fresh Ubuntu VPS for hosting the CRM (and later apps) with Dokploy.
#
#   Step 1, as root, from the provider's console or a root SSH session:
#     SSH_PUBKEY="ssh-ed25519 AAAA... you@pc" bash bootstrap.sh setup
#   Then, from a NEW terminal, check that `ssh deploy@<ip>` and `sudo -v` work.
#
#   Step 2, only after that check:
#     sudo bash bootstrap.sh lock-ssh      # key-only SSH, no root login
#
#   Step 3:
#     sudo bash bootstrap.sh dokploy       # Docker + Dokploy (panel on :3000)
#
# Every step is idempotent and can be re-run.
set -euo pipefail

DEPLOY_USER="${DEPLOY_USER:-deploy}"
TIMEZONE="${TIMEZONE:-Asia/Jerusalem}"
SWAP_SIZE="${SWAP_SIZE:-2G}"

log() { printf '\n\033[1;32m==> %s\033[0m\n' "$*"; }
die() { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }
[ "$(id -u)" -eq 0 ] || die "run as root (or with sudo)"

setup() {
  [ -n "${SSH_PUBKEY:-}" ] || die 'SSH_PUBKEY is empty: SSH_PUBKEY="ssh-ed25519 AAAA..." bash bootstrap.sh setup'
  case "$SSH_PUBKEY" in ssh-ed25519\ *|ssh-rsa\ *|ecdsa-sha2-*) ;; *) die "SSH_PUBKEY does not look like a public key" ;; esac

  log "System update"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -q
  apt-get -yq full-upgrade
  apt-get -yq install ca-certificates curl git ufw fail2ban unattended-upgrades htop jq

  log "Timezone $TIMEZONE"
  timedatectl set-timezone "$TIMEZONE"

  log "User $DEPLOY_USER (sudo, SSH key)"
  id "$DEPLOY_USER" >/dev/null 2>&1 || adduser --disabled-password --gecos "" "$DEPLOY_USER"
  usermod -aG sudo "$DEPLOY_USER"
  # Key-only account: passwordless sudo, since it has no password to type.
  echo "$DEPLOY_USER ALL=(ALL) NOPASSWD:ALL" > "/etc/sudoers.d/90-$DEPLOY_USER"
  chmod 440 "/etc/sudoers.d/90-$DEPLOY_USER"
  install -d -m 700 -o "$DEPLOY_USER" -g "$DEPLOY_USER" "/home/$DEPLOY_USER/.ssh"
  touch "/home/$DEPLOY_USER/.ssh/authorized_keys"
  grep -qxF "$SSH_PUBKEY" "/home/$DEPLOY_USER/.ssh/authorized_keys" || echo "$SSH_PUBKEY" >> "/home/$DEPLOY_USER/.ssh/authorized_keys"
  chown "$DEPLOY_USER:$DEPLOY_USER" "/home/$DEPLOY_USER/.ssh/authorized_keys"
  chmod 600 "/home/$DEPLOY_USER/.ssh/authorized_keys"

  log "Swap $SWAP_SIZE (builds on a small VPS need it)"
  if ! swapon --show | grep -q .; then
    fallocate -l "$SWAP_SIZE" /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
    echo 'vm.swappiness=10' > /etc/sysctl.d/99-swap.conf
    sysctl -q --system
  fi

  log "Firewall: SSH, HTTP, HTTPS (+3000 for the first Dokploy login)"
  ufw default deny incoming
  ufw default allow outgoing
  ufw allow OpenSSH
  ufw allow 80/tcp
  ufw allow 443/tcp
  ufw allow 3000/tcp
  ufw --force enable

  log "fail2ban (bans IPs that brute-force SSH)"
  cat > /etc/fail2ban/jail.local <<'EOF'
[DEFAULT]
bantime  = 1h
findtime = 10m
maxretry = 5

[sshd]
enabled = true
EOF
  systemctl enable --now fail2ban
  systemctl restart fail2ban

  log "Automatic security updates"
  cat > /etc/apt/apt.conf.d/20auto-upgrades <<'EOF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
APT::Periodic::AutocleanInterval "7";
EOF

  log "Done. Keep this session open and test from a NEW terminal:"
  echo "    ssh $DEPLOY_USER@$(curl -fs4 https://ifconfig.me || hostname -I | cut -d' ' -f1)"
  echo "    sudo -v"
  echo "Then run:  sudo bash bootstrap.sh lock-ssh"
}

lock_ssh() {
  [ -s "/home/$DEPLOY_USER/.ssh/authorized_keys" ] || die "no key for $DEPLOY_USER: run 'setup' first"
  log "SSH: keys only, no root login"
  # sshd keeps the FIRST value it reads and includes sshd_config.d/*.conf in
  # name order, so 00- wins over the provider's 50-cloud-init.conf.
  cat > /etc/ssh/sshd_config.d/00-hardening.conf <<'EOF'
PermitRootLogin no
PasswordAuthentication no
KbdInteractiveAuthentication no
PubkeyAuthentication yes
MaxAuthTries 3
X11Forwarding no
EOF
  sshd -t || die "invalid sshd configuration, nothing restarted"
  systemctl restart ssh 2>/dev/null || systemctl restart sshd
  log "Locked. From now on: ssh $DEPLOY_USER@<ip>  (root and passwords are refused)"
}

dokploy() {
  log "Docker log rotation (keeps the 50 GB disk from filling up)"
  install -d /etc/docker
  [ -f /etc/docker/daemon.json ] || cat > /etc/docker/daemon.json <<'EOF'
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" }
}
EOF

  if ! command -v docker >/dev/null 2>&1; then
    log "Docker"
    # Docker's own repository may lag behind a brand-new Ubuntu release:
    # fall back to Ubuntu's packages in that case.
    curl -fsSL https://get.docker.com | sh || apt-get -yq install docker.io docker-compose-v2 docker-buildx
  fi
  systemctl enable --now docker
  usermod -aG docker "$DEPLOY_USER" 2>/dev/null || true

  log "Dokploy"
  curl -sSL https://dokploy.com/install.sh | sh

  log "Open http://<ip>:3000 NOW and create the admin account (the first visitor becomes admin)."
}

case "${1:-}" in
  setup) setup ;;
  lock-ssh) lock_ssh ;;
  dokploy) dokploy ;;
  *) echo "usage: bash bootstrap.sh setup | lock-ssh | dokploy" >&2; exit 2 ;;
esac
