# Example: log report

The flow the component guide walks through, in Go and in Python:

1. `generate` writes chunks of log lines (a source).
2. `grep` keeps the ERROR lines, in two replicas. It is the template
   component, `templates/component-go` or `templates/component-python`.
3. `count` totals the lines per level and writes them as CSV when all
   input is read (a stateful step). Its output is the run's result.

| Path | What it is |
|---|---|
| `generate/`, `count/` | The Go components. |
| `python/` | The same two in Python, run with the Python template's image. |
| `flow.yaml` | The StreamRun. Put your images' digests in it. |
| `test.sh` | Runs the flow in local mode, step by step, in both languages, and checks they give the same result. |

The guide is at https://alpheus-dev.org/docs/components/.
