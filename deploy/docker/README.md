# Docker Notes

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
3. Edit `deploy/docker/docker-compose.yml` and add each modem under `services.api.devices`.
4. Start the stack:

```bash
docker compose -f deploy/docker/docker-compose.yml up --build -d
```

5. Open the UI on port `8081`.
6. Add modems through the UI by running modem scan and saving the discovered device.

## Persistence

- `SQLite` is stored in Docker volume `smsdock-data`.
- SMS are kept permanently.
- Modem events are retained by the backend cleanup policy.
