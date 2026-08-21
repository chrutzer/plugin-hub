# plugin-hub

A small Go API server that loads [Agent Plugins](https://agent-plugins.org/)
from zip files (on a webserver or locally on disk), validates them against
the Agent Plugins v1.0.0 specification, and serves them as a JSON API. Each
loaded plugin's original zip can also be downloaded again.

## How it works

1. A config YAML file (`config.yaml` by default) lists one or more
   **sources**: each is a zip file reachable by URL (`http://`/`https://`)
   or by local path.
2. On startup (and whenever `Reload()` is called), each source's zip is
   fetched/copied into a temporary directory, extracted, and loaded as a
   plugin:
   - `plugin.json` is parsed and validated (required `$schema`/`name`, name
     format, closed schema with unknown fields ignored).
   - `skills/*/SKILL.md` files are discovered and their YAML frontmatter
     validated per the Agent Skills spec.
   - `mcp.json` is parsed and validated (stdio / streamable-http / sse server
     entries); invalid entries are skipped without failing the whole plugin.
3. A failure in one source, skill, or MCP server entry does not affect
   others (per the spec's failure-isolation rules).
4. The server exposes the loaded plugins over HTTP.

## Run

```sh
go run ./cmd/plugin-hub -config config.example.yaml
```

Set the `PORT` environment variable to change the listen port (default `8080`).
Use `-config` to point at a different config file (default `config.yaml`).

## Config

```yaml
sources:
  - testdata/sample-plugin.zip
  - https://example.com/my-plugin.zip
```

## API

| Method | Path                             | Description                          |
| ------ | --------------------------------- | ------------------------------------ |
| GET    | `/`                                | List loaded plugins (summary).       |
| GET    | `/{name}`                          | Full plugin detail incl. skills/MCP. |
| GET    | `/{name}.zip`                      | Download the plugin's original zip.  |

Plugins are keyed by their manifest `name` (from `plugin.json`), not the
source name.

## Notes

- Zip extraction rejects any entry that would escape the destination
  directory (zip-slip protection).
- Not yet implemented: automatic periodic refresh (call `Reload()` yourself
  on a timer if you need it), and MCP server connection/execution — this
  service only discovers and validates plugin metadata.
