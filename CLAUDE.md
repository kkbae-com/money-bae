# CLAUDE.md

money-bae is a personal finance app, structured as a monorepo:

- `tui/` — the original Rust terminal UI application. See `tui/CLAUDE.md`.
- `servers/api/` — a Go + GORM API for a future web/mobile frontend. See `servers/api/CLAUDE.md`.
- `servers/local-dev/` — a docker-compose Postgres for local API development. See `servers/local-dev/README.md`.
- `clients/app/` — a TanStack Router React SPA. See `clients/app/CLAUDE.md`.
- `platform/db/` — Terraform for the shared Azure Postgres server backing both `tui/` and `servers/api/`. See `platform/db/CLAUDE.md`.
- `platform/entra-external-id/` — Terraform for the shared Entra External ID (CIAM) tenant and app registrations. See `platform/entra-external-id/CLAUDE.md`.
- `platform/api/` — CDK app deploying `servers/api/` to AWS App Runner. See `servers/api/CLAUDE.md`'s Deploy section.
- `platform/web-client/` — CDK app deploying `clients/app/` to AWS (S3 + CloudFront).
- `platform/dns/` — CDK app owning the `kkbae.com` Route 53 public hosted zone; consuming stacks (like `platform/web-client`) import the zone by ID. See `platform/dns/README.md`.

Each project's own CLAUDE.md/AGENTS.md/README.md has the details for working within it.
