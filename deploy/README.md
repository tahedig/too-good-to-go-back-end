# Staging backend deployment

GitHub Actions validates the Docker build on pull requests. A successful push
to `develop` publishes `develop` and `develop-<commit-sha>` to GHCR. The staging
server polls the moving tag, verifies the matching immutable tag, and deploys
only when the embedded commit SHA changes.

The server directory is `/home/fpna/apps/backend`. Install:

- `compose.staging.yaml` as `compose.template.yaml`
- `deploy-staging.sh`
- `poll-staging-image.sh`
- a mode `0600` `.env` based on `staging.env.example`

Install the service and timer in `/etc/systemd/system` and enable only the
timer. The API is published on `127.0.0.1:8081`; the frontend reaches it as
`api:8080` over the external `staging-app` Docker network.

Each deployment starts or verifies PostgreSQL and Redis, creates a PostgreSQL
custom-format backup, applies the migrations bundled in the exact API image,
starts the new API, and verifies `/health` both inside the container and from
the host. Only the newest five pre-deployment backups are retained in
`/home/fpna/apps/backend/backups`.

Migration rollback is deliberately not automatic. New migrations must be
backward-compatible with the previous API image. If the new API fails health
checks, the script restores the previous API image but leaves the forward
database migration in place. A failed migration leaves the old API running and
requires inspection before another deployment.

The server needs a read-only GHCR token in `/home/fpna/.docker/config.json`.
Secrets stay in the server `.env`; they are never passed through GitHub Actions
or committed to the repository.
