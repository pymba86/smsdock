# smsdock

## What the VM must provide

- Linux VM in Proxmox.
- USB passthrough for each modem into the VM.
- Stable device names inside the VM:
  - preferred: `/dev/serial/by-id/...`
  - acceptable: custom `udev` aliases such as `/dev/modem-01`

`/dev/ttyUSB*` can change after reboot. Docker should map stable names instead.

## Startup flow

1. Pass USB modems from Proxmox into the Linux VM.
2. Confirm the VM sees stable device names.
3. Edit `docker-compose.yml` and add each modem under `services.api.devices`.
4. Start the stack:

```bash
docker compose up --build -d
```

5. Open the UI on port `8080`.
6. The backend serves both the API and the built frontend from `/app/web` inside the same container.
7. Add modems through the UI by running modem scan and saving the discovered device.

The project uses a single root `Dockerfile` for the final image build.

## GitHub Container Registry

- GitHub Actions builds the Docker image from the root `Dockerfile`.
- Pull requests into `main` validate the image build without publishing.
- Pushes into `main` publish `ghcr.io/pymba86/smsdock:latest` and SHA tags.
- Git tags matching `v*` publish version tags to `ghcr.io/pymba86/smsdock`.
- `docker-compose.yml` defaults to `ghcr.io/pymba86/smsdock:latest`.
- To pin a version, start compose with `SMSDOCK_IMAGE=ghcr.io/pymba86/smsdock:v1.0.0 docker compose up -d`.
- For production from registry, use `docker-compose.prod.yml`:

```bash
docker compose -f docker-compose.prod.yml up -d
```

- To pin a production version, use:

```bash
SMSDOCK_IMAGE=ghcr.io/pymba86/smsdock:v1.0.0 docker compose -f docker-compose.prod.yml up -d
```

## Persistence

- `SQLite` is stored in Docker volume `smsdock-data`.
- SMS are kept permanently.
- Modem events are retained by the backend cleanup policy.
