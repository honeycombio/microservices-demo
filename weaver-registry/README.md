# Online Boutique Semantic Convention Registry

This directory contains the custom [OpenTelemetry Weaver](https://github.com/open-telemetry/weaver) registry for the Online Boutique demo application. It defines application-specific attributes, span definitions, and span events that extend the upstream OTel semantic conventions.

## Structure

```
weaver-registry/
├── registry_manifest.yaml   # Registry metadata and OTel semconv dependency
└── registry/
    ├── app.yaml             # app.* attribute groups (user, order, catalog, runtime, ads)
    ├── checkout-events.yaml # Span event attribute groups for checkoutservice lifecycle events
    └── spans.yaml           # Span definitions for each instrumented service
```

## Prerequisites

Install the Weaver CLI:

```bash
cargo install weaver-forge
```

Or download a binary from the [Weaver releases page](https://github.com/open-telemetry/weaver/releases).

Verify installation:

```bash
weaver --version
```

## Commands

All commands are run from the **repository root**.

### Validate the registry

```bash
weaver registry check --registry weaver-registry
```

A clean registry prints `✔ No 'after_resolution' policy violation` and exits 0.

To enable stricter future validation rules (recommended when making changes):

```bash
weaver registry check --registry weaver-registry --future
```

### View registry statistics

```bash
weaver registry stats --registry weaver-registry
```

Prints a breakdown of groups, attributes, span kinds, requirement levels, and stability.

### Resolve and inspect the registry

Outputs the fully resolved registry as JSON, with all `ref:` lookups and OTel dependency attributes inlined:

```bash
weaver registry resolve --registry weaver-registry
```

### Generate artifacts from the registry

```bash
weaver registry generate \
  --registry weaver-registry \
  --templates <path-to-templates> \
  <output-dir>
```

See the [Weaver template guide](https://github.com/open-telemetry/weaver/blob/main/docs/usage.md) for how to write Jinja2 templates that produce Go structs, Markdown docs, etc.

## Adding or modifying attributes

1. **New application attribute** — add it to the appropriate group in `registry/app.yaml`. Use an `id` without the namespace prefix (e.g. `user_id`, not `app.user_id`), and include `type`, `stability`, `brief`, `examples`, and `requirement_level`.

2. **New span** — add a group with `type: span` to `registry/spans.yaml`. Reference OTel standard attributes with `ref:` and inline any custom attributes with their full dotted name as `id`.

3. **New span event** — add an attribute group to `registry/checkout-events.yaml` and reference the event name in the relevant span's `events:` list.

After any change, run the check command to validate:

```bash
weaver registry check --registry weaver-registry
```

## Valid field values

| Field | Allowed values |
|---|---|
| `requirement_level` | `required`, `recommended`, `opt_in` |
| `stability` | `development`, `release_candidate`, `stable` |
| `type` (attribute) | `string`, `int`, `double`, `boolean`, `string[]`, `int[]`, `double[]`, `boolean[]` |
| `span_kind` | `server`, `client`, `producer`, `consumer`, `internal` |

## Dependency on OTel semconv

The manifest pins the upstream OTel semantic conventions registry at `v1.34.0`:

```yaml
dependencies:
  - name: otel
    registry_path: https://github.com/open-telemetry/semantic-conventions@v1.34.0[model]
```

Attributes referenced via `ref:` in `spans.yaml` (e.g. `rpc.system`, `db.query.text`, `http.request.method`) are resolved from this dependency at check/generate time. Weaver caches the downloaded registry locally in `~/.weaver/`.

To update the pinned version, change the `@v1.34.0` tag in `registry_manifest.yaml` and re-run the check.
