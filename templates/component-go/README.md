# Component template: Go

A complete Alpheus component in Go: `grep`, which keeps the lines of each
input chunk that contain a text. Copy this folder to start your own.

| File | What it is |
|---|---|
| `main.go` | The component. Replace `handle` with your own; keep `sdk.Run` in `main`. |
| `main_test.go` | A test in local mode: input chunks from a folder, output to another. |
| `Dockerfile` | Builds the component as a static binary on `scratch`. |

Start your own module from it:

```console
$ cp -r templates/component-go my-component && cd my-component
$ go mod init example.com/my-component
$ go get alpheus-dev.org/alpheus@latest
$ go test ./...
$ podman build -t registry.example/my-component:dev .
```

Try it without Kubernetes in local mode:

```console
$ ALPHEUS_MODE=local ALPHEUS_LOCAL_INPUT=./in ALPHEUS_LOCAL_OUTPUT=./out \
    ALPHEUS_SETTINGS='{"match": "ERROR"}' go run .
```

The guide is at https://alpheus-dev.org/docs/components/.
