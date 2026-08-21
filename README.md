# plugin-hub

A small Go API server that loads [Agent Plugins](https://agent-plugins.org/)
from zip files (on a webserver or locally on disk), validates them against
the Agent Plugins v1.0.0 specification, and serves them as a JSON API. Each
loaded plugin's original zip can also be downloaded again.

## How it works

1. A config YAML file (`config.yaml` by default) lists one or more
   **sources**: each is a zip file reachable by URL (`http://`/`https://`),
   a local zip file path, or a local directory path (zipped on the fly).
2. On startup (and whenever `Reload()` is called), each source's zip is
   fetched/copied into a temporary directory and extracted. If the extracted
   contents contain `plugin.json` (directly, or inside a single top-level
   subdirectory), it's loaded as a plugin. Otherwise it's treated as a
   **bundle**: any `*.zip` files found inside (directly, or inside a single
   top-level subdirectory) are recursively resolved the same way, so a zip of
   zips of plugins — or a zip of zips of zips of plugins, and so on — is
   supported.
3. Each resolved plugin is loaded:
   - `plugin.json` is parsed and validated (required `$schema`/`name`, name
     format, closed schema with unknown fields ignored).
   - `skills/*/SKILL.md` files are discovered and their YAML frontmatter
     validated per the Agent Skills spec.
   - `mcp.json` is parsed and validated (stdio / streamable-http / sse server
     entries); invalid entries are skipped without failing the whole plugin.
4. A failure in one source, bundle entry, skill, or MCP server entry does not
   affect others (per the spec's failure-isolation rules).
5. The server exposes the loaded plugins over HTTP.

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
  - testdata/sample-plugin
  - https://example.com/my-plugin.zip
  - https://example.com/plugin-bundle.zip
```

## API

| Method | Path                             | Description                          |
| ------ | --------------------------------- | ------------------------------------ |
| GET    | `/`                                | List loaded plugins (summary).       |
| GET    | `/{name}`                          | Full plugin detail incl. skills/MCP. |
| GET    | `/{name}.zip`                      | Download the plugin's original zip.  |

Plugins are keyed by their manifest `name` (from `plugin.json`), not the
source name. For plugins loaded from a bundle, `/{name}.zip` downloads that
plugin's own nested zip, not the outer bundle.

## Notes

- Zip extraction rejects any entry that would escape the destination
  directory (zip-slip protection), at every nesting level.
- Not yet implemented: automatic periodic refresh (call `Reload()` yourself
  on a timer if you need it), and MCP server connection/execution — this
  service only discovers and validates plugin metadata.
