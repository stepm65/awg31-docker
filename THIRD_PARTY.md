# Third-party components

This repository contains its own Go daemon (`awgd`), web console and Docker packaging, licensed as indicated in `LICENSE`. It does **not** vendor `amneziawg-go` or `amneziawg-tools`, and it has no third-party Go modules (standard library only).

At image build time the `Dockerfile` downloads sources from the original AmneziaWG projects and keeps their respective upstream licences. Both the tag and the commit hash are pinned and checked:

- https://github.com/amnezia-vpn/amneziawg-go — userspace implementation of the protocol (tag `v3.1.20260828`); see the upstream licence file.
- https://github.com/amnezia-vpn/amneziawg-tools — `awg` and `awg-quick` utilities (tag `v3.1.20260812`); see the upstream licence file.
- Alpine Linux packages (`bash`, `iproute2`, `iptables`, `openresolv`, `ca-certificates`) in the final image keep their own licences.

The look of the web console follows the AWG Console panel from https://github.com/stepm65/amneziawg31-installer (same author); its colour tokens were reimplemented here, no panel code is included.

- `scripts/routeros-app-yaml.schema.json` is the RouterOS `/app` YAML schema by the tikoci project (https://github.com/tikoci/restraml), copied unchanged from routeros-aligorov for CI validation; it keeps its upstream licence.
- `entrypoint.sh` contains the `ip rule` and `resolv.conf` fixes for RouterOS containers from `a-amneziawg-mikrotik` (routeros-aligorov, same author).

The project is not affiliated with the AmneziaVPN developers.

## Fonts bundled with the web console

The console serves its typefaces from the binary (`internal/web/static/fonts/`, embedded with `//go:embed`); nothing is loaded from a CDN, which is also what the `font-src 'self'` Content-Security-Policy enforces.

- **Exo 2** — Copyright 2013 The Exo 2 Project Authors (https://github.com/googlefonts/Exo-2.0), SIL Open Font License 1.1, see `internal/web/static/fonts/OFL-Exo2.txt`.
- **JetBrains Mono** — Copyright 2020 The JetBrains Mono Project Authors (https://github.com/JetBrains/JetBrainsMono), SIL Open Font License 1.1, see `internal/web/static/fonts/OFL-JetBrainsMono.txt`.

The licence texts are embedded with the fonts, so they travel inside the binary and the image and are served by the console at `/fonts/OFL-Exo2.txt` and `/fonts/OFL-JetBrainsMono.txt`.

The files are the unmodified variable `woff2` subsets (latin, cyrillic) published as `@fontsource-variable/exo-2` and `@fontsource-variable/jetbrains-mono` 5.3.0, copied from the panel repository.

## Service icons in the routing editor

The routing-rule editor labels rules with the logos of popular services. Their SVG paths are taken from **Simple Icons** 16.38.1 (https://github.com/simple-icons/simple-icons), released under CC0 1.0, and compiled into `internal/web/static/services.js` by `scripts/gen-services.mjs`; nothing is fetched at runtime. The logos are trademarks of their owners and are used only to identify the service a rule refers to.
