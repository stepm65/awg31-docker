#!/usr/bin/env bash
# Сборка образов awg31 в .tar для офлайн-импорта на MikroTik (/container/add file=...).
#   usage: scripts/build-routeros.sh [платформы]
#   по умолчанию: linux/arm64 linux/arm/v7 (+ linux/amd64 для CHR, если указать)
# Результат: dist/awg31-<версия>-<платформа>.tar и SHA256SUMS.
# Нужен Docker с buildx и эмуляцией QEMU (docker run --privileged --rm tonistiigi/binfmt --install all).
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="$(tr -d ' \n' < VERSION)"
PLATFORMS=("${@:-linux/arm64 linux/arm/v7}")
read -r -a PLATFORMS <<< "${PLATFORMS[*]}"
mkdir -p dist
for p in "${PLATFORMS[@]}"; do
    case "$p" in
        linux/arm64|linux/arm/v7|linux/amd64) ;;
        *) echo "платформа $p не поддерживается (arm64, arm/v7, amd64)" >&2; exit 1 ;;
    esac
    out="dist/awg31-${VERSION}-${p//\//-}.tar"
    echo ">> $p -> $out"
    # type=docker даёт архив в формате «docker save»: его понимает /container/add file=
    docker buildx build --platform "$p" --build-arg VERSION="$VERSION" \
        -t "awg31:${VERSION}" --output "type=docker,dest=${out}" .
done
(cd dist && sha256sum awg31-"${VERSION}"-*.tar > SHA256SUMS && cat SHA256SUMS)
