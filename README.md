# tfs-cli

CLI for working with TFS/Azure DevOps Server work items (create, update, search, WIQL, and more).

## Features
- Run WIQL and list matching work items.
- Create and update work items (including comments).
- Search by title/description.
- Show details and child items.
- List work item types and resolve your identity.
- JSON output by default, with optional text tables.

## Requirements
- Go 1.21+ (for building from source).
- A TFS/Azure DevOps Server instance and a PAT token.

## Install
Build a local binary:

```bash
go build -o tfs .
```

Run it from the repo root:

```bash
./tfs --help
```

## Quickstart
Configure the CLI, then create a backlog item and a child task. The examples below use common English work item names:
- Product Backlog Item
- Task

```bash
./tfs config set --base-url "https://dev.azure.com/your-org" --project "YourProject" --pat "YOUR_PAT"

./tfs create --type "Product Backlog Item" --title "Export data to CSV" \
  --set "System.Description=Allow users to export filtered data to CSV"

./tfs create --type "Task" --title "Build export endpoint" --parent 123 \
  --set $'System.Description=Implement the API endpoint\n\nResult: CSV returned in response' \
  --set "Microsoft.VSTS.Common.Activity=Development" \
  --set "Microsoft.VSTS.Scheduling.RemainingWork=4" \
  --set "Microsoft.VSTS.Scheduling.OriginalEstimate=4"
```

## Configuration
You can configure the CLI via a config file, environment variables, or flags.

### Config file
Save config values:

```bash
./tfs config set --base-url "https://dev.azure.com/your-org" --project "YourProject" --pat "YOUR_PAT"
```

View config (PAT redacted):

```bash
./tfs config view
```

The config file is stored in the OS user config directory (for example, `~/.config/tfs/config.json` on Linux, `%AppData%\\tfs\\config.json` on Windows).

### Environment variables
- `TFS_BASE_URL`
- `TFS_PROJECT`
- `TFS_PAT`

### Precedence
Flags override environment variables, which override the config file.

### Base URL format
Set `--base-url` (or `TFS_BASE_URL`) to the organization/collection root:
- Azure DevOps Services: `https://dev.azure.com/{org}`
- TFS on-prem: `http://server:8080/tfs/{collection}`

If the URL ends with the project name, the CLI will normalize it automatically.

## Usage
Run `./tfs --help` for the full command list. Main commands:

- `wiql` - run a WIQL query and list items
- `view` - show a work item by ID
- `update` - update fields or add a comment
- `create` - create a work item
- `search` - search by title/description
- `my` - list your assigned items
- `show` - show details plus child items
- `types` - list work item types
- `whoami` - show the resolved identity from the PAT
- `config` - view/set stored config

## Examples
The examples below assume these English type names and field references:
- Work item types: `Product Backlog Item`, `Task`
- Activity field: `Microsoft.VSTS.Common.Activity` (value: `Development`)
- Remaining work: `Microsoft.VSTS.Scheduling.RemainingWork`
- Original estimate: `Microsoft.VSTS.Scheduling.OriginalEstimate`

List work item types (text output):

```bash
./tfs types --json=false
```

Create a work item:

```bash
./tfs create --type "Product Backlog Item" --title "Add bulk export" \
  --set "System.Description=Export data to CSV"
```

Create a child task:

```bash
./tfs create --type "Task" --title "Implement export endpoint" --parent 123 \
  --set $'System.Description=Implement the API endpoint\n\nResult: CSV returned in response' \
  --set "Microsoft.VSTS.Common.Activity=Development" \
  --set "Microsoft.VSTS.Scheduling.RemainingWork=4" \
  --set "Microsoft.VSTS.Scheduling.OriginalEstimate=4"
```

Update fields and add a comment:

```bash
./tfs update 123 --set "System.Title=Export supports filters" --add-comment "Updated scope"
```

Show details and children:

```bash
./tfs show 123
```

Search:

```bash
./tfs search --query "export csv" --top 20
```

Run a WIQL query:

```bash
./tfs wiql "SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'New'" --top 50
```

## Output
- Most commands output JSON by default.
- Use `--json=false` to get text tables where supported.
- `my` and `show` default to text unless `--json` is explicitly provided.

## Development
Run tests:

```bash
go test ./...
```

Integration tests require a live instance and environment variables:

```bash
TFS_BASE_URL=... \
TFS_PROJECT=... \
TFS_PAT=... \
TFS_WIT_TYPE=... \
go test -tags=integration ./internal/integration
```

Optional integration variables:
- `TFS_TASK_TYPE` (default in code: `Задача`)
- `TFS_TASK_ACTIVITY_NAME` (default in code: `Активность`)
- `TFS_TASK_ACTIVITY_VALUE` (default in code: `Разработка`)
- `TFS_TASK_REMAINING_WORK_NAME` (default in code: `Оставшаяся работа`)
- `TFS_ASSIGNED_TO`
- `TFS_INSECURE` (set to non-empty to skip TLS verification)

## Notes
- The CLI uses Azure DevOps REST API version 6.0.
- For API background and examples (in Russian), see `tfs_api.md`.
