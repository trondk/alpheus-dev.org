# Component template: Python

A complete Alpheus component in Python: `grep`, which keeps the lines of
each input chunk that contain a text. Copy this folder to start your own.

| File | What it is |
|---|---|
| `component.py` | The component. Replace `handle` with your own; keep `alpheus.run` at the end. |
| `alpheus/` | A small implementation of the component contract: the gRPC calls to the agent, and local mode. It is a reference, not a maintained SDK. |
| `alpheus/component.proto` | The contract, a copy of `proto/component.proto`. The Dockerfile generates the Python stubs from it. |
| `requirements.txt` | grpcio and protobuf, pinned. |
| `Dockerfile` | Builds the image, running as uid 65532 with a read-only root. |
| `test.sh` | Runs the component in local mode in its image and checks the result. |

```console
$ cp -r templates/component-python my-component && cd my-component
$ ./test.sh
$ podman build -t registry.example/my-component:dev .
```

The guide is at https://alpheus-dev.org/docs/components/.
