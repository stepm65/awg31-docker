# syntax=docker/dockerfile:1
#
# Образ awg31: amneziawg-go (userspace-реализация) + awg/awg-quick + управляющий демон awgd.
# Сборка: docker build -t awg31 .
# Версия берётся из файла VERSION; --build-arg VERSION=… её переопределяет (CI).
# Платформы: linux/amd64, linux/arm64, linux/arm/v7 (MikroTik arm64 и arm32, см. docs/MIKROTIK.md).

ARG GO_VERSION=1.25-alpine3.21
ARG ALPINE_VERSION=3.21

# Версии upstream фиксируются и по тегу, и по коммиту: если тег в upstream
# переставят, сборка упадёт, а не соберёт другой код.
ARG AWG_GO_REF=v3.1.20260828
ARG AWG_GO_COMMIT=b5928efb6ca19f0153958460c3d141f04abc5c2e
ARG AWG_TOOLS_REF=v3.1.20260812
ARG AWG_TOOLS_COMMIT=ee0f0a9aa34ff0a0da4b3433b9512781cfe02843

# ---------- amneziawg-go ----------
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build-awg-go
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG AWG_GO_REF
ARG AWG_GO_COMMIT
RUN apk add --no-cache git
WORKDIR /src
RUN git clone --depth=1 --branch "${AWG_GO_REF}" https://github.com/amnezia-vpn/amneziawg-go.git . \
 && test "$(git rev-parse HEAD)" = "${AWG_GO_COMMIT}"
RUN set -eux; \
    GOARM=""; \
    if [ "$TARGETARCH" = "arm" ]; then GOARM="${TARGETVARIANT#v}"; fi; \
    CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" GOARM="$GOARM" \
    go build -trimpath -ldflags="-s -w" -o /out/amneziawg-go .

# ---------- awgd (этот репозиторий) ----------
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build-awgd
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=
WORKDIR /src
COPY go.mod VERSION ./
COPY cmd ./cmd
COPY internal ./internal
RUN set -eux; \
    GOARM=""; \
    if [ "$TARGETARCH" = "arm" ]; then GOARM="${TARGETVARIANT#v}"; fi; \
    V="${VERSION:-$(tr -d ' \n' < VERSION)}"; \
    CGO_ENABLED=0 GOOS="$TARGETOS" GOARCH="$TARGETARCH" GOARM="$GOARM" \
    go build -trimpath -ldflags="-s -w -X awg31/internal/version.Version=${V}" -o /out/awgd ./cmd/awgd

# ---------- awg + awg-quick (C, собирается под целевую архитектуру) ----------
FROM alpine:${ALPINE_VERSION} AS build-tools
ARG AWG_TOOLS_REF
ARG AWG_TOOLS_COMMIT
RUN apk add --no-cache git make build-base linux-headers bash
WORKDIR /src
RUN git clone --depth=1 --branch "${AWG_TOOLS_REF}" https://github.com/amnezia-vpn/amneziawg-tools.git . \
 && test "$(git rev-parse HEAD)" = "${AWG_TOOLS_COMMIT}"
RUN make -C src && strip src/wg

# ---------- итоговый образ ----------
FROM alpine:${ALPINE_VERSION}
# Метка версии заполняется только при --build-arg VERSION (так делает CI);
# версия самого приложения всегда берётся из файла VERSION.
ARG VERSION=
LABEL org.opencontainers.image.title="awg31" \
      org.opencontainers.image.description="AmneziaWG 3.1 в Docker с WebUI" \
      org.opencontainers.image.version="${VERSION}"

# iptables-legacy: он работает и на ядрах без nf_tables (например, на RouterOS).
# conntrack-tools: мониторинг соединений, если ядро не отдаёт /proc/net/nf_conntrack.
# tini — PID 1: подбирает осиротевшие процессы (amneziawg-go уходит в фон и после
# выключения туннеля оставался зомби у awgd) и передаёт сигналы остановки.
RUN apk add --no-cache bash iproute2 iptables iptables-legacy openresolv ca-certificates conntrack-tools tini \
 && for t in iptables iptables-save iptables-restore ip6tables ip6tables-save ip6tables-restore; do \
      ln -sf "/usr/sbin/${t}-legacy" "/usr/sbin/${t}"; \
    done \
 && mkdir -p /data /run/awg31 \
 && chmod 700 /data /run/awg31

COPY --from=build-awg-go /out/amneziawg-go /usr/bin/amneziawg-go
COPY --from=build-awgd /out/awgd /usr/bin/awgd
COPY --from=build-tools /src/src/wg /usr/bin/awg
COPY --from=build-tools /src/src/wg-quick/linux.bash /usr/bin/awg-quick
COPY entrypoint.sh /entrypoint.sh
RUN chmod 0755 /usr/bin/amneziawg-go /usr/bin/awgd /usr/bin/awg /usr/bin/awg-quick /entrypoint.sh

ENV DATA_DIR=/data \
    RUN_DIR=/run/awg31 \
    UI_LISTEN=:8443 \
    UI_TLS=auto \
    AWG_AUTO_ROUTE=on \
    WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go

VOLUME ["/data"]
EXPOSE 8443/tcp

HEALTHCHECK --interval=60s --timeout=5s --start-period=15s --retries=3 \
    CMD ["/usr/bin/awgd", "healthcheck"]

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]
