# local-mlx

A small Go client and CLI for talking to an [`mlx_lm.server`](https://github.com/ml-explore/mlx-lm)
(OpenAI-compatible API) running on a Mac. It is designed to run on a Raspberry Pi
(or the Mac itself) and reach the Mac's model server over the LAN, aimed at
automation/agent use.

- Reusable client: package [`mlx`](./mlx) — `Models`, `Chat`, `ChatStream`.
- CLI: [`cmd/mlx`](./cmd/mlx) — `models`, `run`, `chat`, `heartbeat`.
- Standard library only (no third-party Go deps), so it cross-compiles to the Pi
  cleanly and ships in a tiny distroless container.

## CLI

```
mlx [--host h] [--model m] <command> [args]
```

Global flags must precede the command.

| Command | Purpose |
|---------|---------|
| `models` | List the models the server has available. |
| `run [prompt]` | One-shot completion. Prompt comes from the argument or stdin. Pipeable — the automation building block. |
| `chat` | Interactive streaming REPL (`/model`, `/models`, `/reset`, `/quit`). |
| `heartbeat [--interval d]` | Long-lived loop that pings the server and logs status. The container's default command. |

### Configuration

Resolved in order: `--host` flag → `MLX_HOST` env → `localhost:8080`. The default
model is `MLX_MODEL`, else the server's first reported model.

```sh
export MLX_HOST=192.168.1.50:8080
echo "say hi in three words" | mlx run
mlx models
mlx chat
```

## Build & test (Bazel)

Everything builds and tests with Bazel (via `bazelisk`):

```sh
bazel test //...              # unit tests (nogo/go-vet runs inside the build)
bazel run //cmd/mlx -- models # run the CLI
bazel run //:buildifier       # format BUILD/.bzl/MODULE files
bazel run //:gazelle          # regenerate BUILD files after adding/removing Go files
```

### Cross-compile for the Raspberry Pi

```sh
bazel build //cmd/mlx --platforms=@rules_go//go/toolchain:linux_arm64
```

## Container image

A multi-arch image (linux/amd64 + linux/arm64) is published to
`ghcr.io/omargallob/local-mlx` by CI:

- `:latest` and `:sha-<short>` on every push to `master`.
- `:vX.Y.Z` when a release is cut.

Build it locally with `bazel build //cmd/mlx:image_index`. The entrypoint is
`mlx heartbeat`; override the args (or `--entrypoint`) to run other subcommands.

## Deploy on the Pi (docker compose)

```sh
cd deploy
cp compose.env.example compose.env    # edit MLX_HOST to the Mac's LAN IP
docker compose --env-file compose.env up -d
docker compose logs -f                 # should show it reaching the Mac
```

`docker compose pull` fetches the arm64 variant automatically on the Pi.

## Connectivity (Pi → Mac)

- The server should bind `0.0.0.0` so the Pi can reach it (mlx_lm.server
  `--host 0.0.0.0`).
- Find the Mac's LAN IP: `ipconfig getifaddr en0` (or `en1` on Wi-Fi).
- If the Pi can't connect, check the macOS firewall allows inbound on port 8080.

## Contributing

Commits follow [Conventional Commits](https://www.conventionalcommits.org/)
(`feat:`, `fix:`, `feat!:` …) — [Release Please](https://github.com/googleapis/release-please)
uses them to version the project and generate `CHANGELOG.md`. Work lands via PRs
to `master` (squash-merged; direct pushes are blocked).
