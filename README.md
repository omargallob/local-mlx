# local-mlx

A small Go client and CLI for talking to an [`mlx_lm.server`](https://github.com/ml-explore/mlx-lm)
(OpenAI-compatible API) running on a Mac. It is built to run on a Raspberry Pi
(or the Mac itself) and reach the Mac's model server over the LAN, aimed at
automation/agent use.

The reusable client lives in package [`mlx`](./mlx); the CLI lives in
[`cmd/mlx`](./cmd/mlx). Standard library only, so it cross-compiles to the Pi
with no third-party Go dependencies.

## Build & test (Bazel)

Everything is built and tested with Bazel (via `bazelisk`):

```sh
bazel test //...              # unit tests (nogo/go-vet runs inside the build)
bazel run //:buildifier       # format BUILD/.bzl/MODULE files
bazel run //:gazelle          # regenerate BUILD files after adding/removing Go files
```

## Configuration

The client finds the Mac's server via (in order): the `--host` flag, the
`MLX_HOST` env var, else `localhost:8080`. The default model comes from
`MLX_MODEL`, else the first model the server reports.

## Status

Bootstrap: `mlx` client package + Bazel build/test + CI. The CLI, container
image, release automation, and Pi deploy land in follow-up milestones — see the
repository's GitHub issues/milestones.
