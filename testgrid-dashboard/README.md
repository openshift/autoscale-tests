# Local TestGrid (aws-hpa / autonode 4.21)

A self-contained local TestGrid stack: a **pipeline** service refreshes the data
every 60 min, the Go **API server** serves it, and the **web frontend** renders
it. Shows the `autoscale-health` dashboard (`hpa` and `autonode` tabs).

## Run it

1. **Prereqs (once):** Docker or Podman + Compose, and
   `gcloud auth application-default login` (the updater needs a credential
   present to read the public bucket).
2. Clone this repo and `cd testgrid-dashboard`.
3. `docker compose up --build -d`
4. Open http://localhost:8081

Stop / clean up: `docker compose down`.

The pipeline populates the grid on its first cycle (~1 min) and then refreshes it
in place every 60 min. By default the grid shows only failed/flaky tests (uncheck
the box at the top to see all rows).

## Podman (Fedora, no Docker daemon)

```bash
systemctl --user start podman.socket
export DOCKER_HOST="unix:///run/user/$(id -u)/podman/podman.sock"
docker compose up --build -d
```

## API_URL caveat

The frontend bakes `API_URL=http://localhost:8080` into its bundle at build time,
so **open the app from the same machine** running the containers. To serve other
machines, rebuild with a reachable address:

```bash
docker compose build --build-arg API_URL=http://my-host:8080 frontend
docker compose up -d
```
