# Security notes

- Never share a profile `.conf`, a subscription link, a private key or a preshared key: each of them can grant VPN access. The panel's subscription link contains the client's private key in effect, so treat it like a password.
- The console listens on `127.0.0.1` of the Docker host by default. Do not publish its port to the internet; if you need remote access, put it behind a properly secured HTTPS reverse proxy and set `TRUSTED_PROXIES`.
- The container needs `NET_ADMIN` and `/dev/net/tun` and runs as root inside the container (`awg-quick` manages interfaces and routes). Capabilities are limited to `NET_ADMIN`, `no-new-privileges` is on.
- Do not commit the contents of the data volume (`/data`: profiles, `state.json`, TLS key), secret files or `.conf` files. The repository `.gitignore` and `.dockerignore` already exclude them.
- The project has not been independently security-audited, and the Docker image build and a real tunnel have not been exercised in the development environment; see "Остаточные риски" in [docs/SECURITY.md](docs/SECURITY.md).
- Report vulnerabilities privately to the repository owner through GitHub (Security → Report a vulnerability) instead of a public issue.

The full list of problems found in the prototype, how each is fixed here and which test pins it down: [docs/SECURITY.md](docs/SECURITY.md) (Russian).
