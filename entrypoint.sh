#!/usr/bin/env /bin/sh
# Copyright (C) 2026 Yota Hamada
# SPDX-License-Identifier: GPL-3.0-or-later


# Check if both DOCKER_GID is not -1. This indicates the desire for a docker group
if [ "$DOCKER_GID" != "-1" ]; then
  if ! getent group docker >/dev/null; then
    echo "Creating docker group with GID ${DOCKER_GID}"
    addgroup -g ${DOCKER_GID} docker
    usermod -a -G docker ayatsuri
  fi 

  echo "Changing docker group GID to ${DOCKER_GID}"
  groupmod -o -g "$DOCKER_GID" docker
fi

CURRENT_UID=$(id -u ayatsuri 2>/dev/null || echo -1)
CURRENT_GID=$(getent group ayatsuri | cut -d: -f3 2>/dev/null || echo -1)

if [ "$CURRENT_UID" != "$PUID" ] || [ "$CURRENT_GID" != "$PGID" ]; then
    groupmod -o -g "$PGID" ayatsuri
    usermod -o -u "$PUID" ayatsuri
fi

mkdir -p ${AYATSURI_HOME:-/var/lib/ayatsuri}

# If AYATSURI_HOME is not set, try to guess if the legacy /home directory is being
# used. If so set the HOME to /home/ayatsuri. Otherwise force the /var/lib/ayatsuri directory
# as AYATSURI_HOME
if [ -z "$AYATSURI_HOME" ]; then
  # For ease of use set AYATSURI_HOME to /var/lib/ayatsuri so all data is located in a
  # single directory following FHS conventions
  export AYATSURI_HOME=/var/lib/ayatsuri
fi

# Run all scripts in /etc/custom-init.d. It assumes that all scripts are
# executable
if [ -d /etc/custom-init.d ]; then
  for f in /etc/custom-init.d/*; do
    if [ -x "$f" ]; then
      echo "Running $f"
      $f
    fi
  done
fi

# If DOCKER_GID is not -1 set RUN_GID to DOCKER_GID otherwise set to PGID
if [ "$DOCKER_GID" != "-1" ]; then
  RUN_GID=$DOCKER_GID
else
  RUN_GID=$PGID
fi

# Run the command as the ayatsuri user and optionally the docker group.
# -E preserves env vars (AYATSURI_HOME, etc.); -H overrides HOME to the target user's
# home directory so that ~ expands correctly (see #1698).
exec sudo -E -H -n -u "#${PUID}" -g "#${RUN_GID}" -- "$@"
