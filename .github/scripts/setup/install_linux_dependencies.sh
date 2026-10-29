#!/usr/bin/env bash
set -Eeuo pipefail

packages=(gcc libgl1-mesa-dev libegl1-mesa-dev libgles2-mesa-dev libx11-dev xorg-dev libasound2-dev libopenal-dev)
case "${APT_PROFILE:-}" in
  web)
    packages+=(binaryen brotli unzip)
    ;;
  native)
    packages+=(unzip)
    ;;
  *)
    echo "::error::Unknown APT profile: ${APT_PROFILE:-<empty>}"
    exit 2
    ;;
esac

retry() {
  local attempts="$1"
  local delay_seconds="$2"
  local description="$3"
  shift 3

  local attempt
  local rc=0

  for ((attempt = 1; attempt <= attempts; attempt++)); do
    echo "[info] ${description}: attempt ${attempt}/${attempts}"

    if "$@"; then
      return 0
    else
      rc=$?
    fi

    if ((attempt == attempts)); then
      echo "::error::${description} failed after ${attempts} attempts (exit ${rc})"
      return "$rc"
    fi

    echo "::warning::${description} failed with exit ${rc}; retrying in ${delay_seconds} seconds"
    sleep "$delay_seconds"
  done
}

apt_acquire_options=(
  -o Acquire::Retries=3
  -o Acquire::http::Timeout=30
  -o Acquire::https::Timeout=30
)

update_package_index() {
  sudo timeout \
    --signal=TERM \
    --kill-after=15s \
    120s \
    apt-get "${apt_acquire_options[@]}" update
}

install_packages() {
  sudo env DEBIAN_FRONTEND=noninteractive \
    timeout \
      --signal=TERM \
      --kill-after=15s \
      300s \
      apt-get \
        "${apt_acquire_options[@]}" \
        -o DPkg::Lock::Timeout=60 \
        install \
        --yes \
        "${packages[@]}"
}

retry 2 10 "APT package index update" update_package_index
retry 2 10 "APT dependency installation" install_packages
