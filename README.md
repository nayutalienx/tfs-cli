# tfs-cli

CLI for working with TFS/Azure DevOps Server work items (create, update, delete, search, WIQL, and more).

## Features
- Run WIQL and list matching work items.
- Create, update, and delete work items (including comments).
- Search by title/description.
- Show details and child items.
- List work item types and resolve your identity.
- Create Git pull requests in TFS/Azure DevOps Server.
- Show Git pull request details by URL or ID (repo, branches, work items, comments).
- Post comment threads on pull requests (inline, stdin, or file input).
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

./tfs create --type "Product Backlog Item" --title "Add report generation" \
  --set "System.Description=Allow users to generate and download reports"

./tfs create --type "Task" --title "Implement report endpoint" --parent 123 \
  --set $'System.Description=Implement the API endpoint\n\nResult: Report file is returned in the response' \
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
- `delete` - delete a work item; add `--destroy` to attempt permanent removal when the PAT has destroy permission
- `search` - search by title/description
- `my` - list your assigned items
- `show` - show details plus child items
- `pr create` - create a Git pull request
- `pr show` - show pull request details (repo, branches, title, work items, comments)
- `pr comment` - post a comment thread on a pull request
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
./tfs create --type "Product Backlog Item" --title "Add report generation" \
  --set "System.Description=Allow users to generate reports"
```

Create a child task:

```bash
./tfs create --type "Task" --title "Implement report endpoint" --parent 123 \
  --set $'System.Description=Implement the API endpoint\n\nResult: Report file is returned in the response' \
  --set "Microsoft.VSTS.Common.Activity=Development" \
  --set "Microsoft.VSTS.Scheduling.RemainingWork=4" \
  --set "Microsoft.VSTS.Scheduling.OriginalEstimate=4"
```

Update fields and add a comment:

```bash
./tfs update 123 --set "System.Title=Report generation supports filters" --add-comment "Updated scope"
```

Delete a work item:

```bash
./tfs delete 123 --yes
```

Permanently delete a work item when the PAT has destroy permission:

```bash
./tfs delete 123 --destroy --yes
```

Create a pull request:

```bash
./tfs pr create --repository "sample-service" \
  --source "feat/update-report-workflow" \
  --target "develop" \
  --title "Update report workflow example" \
  --description "Documentation examples were refreshed with neutral placeholder values" \
  --work-item 12345 \
  --work-item 12346 \
  --auto-complete
```

If you pass `--work-item`, the CLI links those work items to the PR. `--auto-complete` is optional and disabled by default.

Show a pull request by URL:

```bash
./tfs pr show "https://dev.azure.com/your-org/YourProject/_git/your-repo/pullrequest/42"
```

Show a pull request by ID with explicit repository:

```bash
./tfs pr show 42 --repository "your-repo"
```

The `pr show` command prints the repository name, source and target branches, PR title, description, linked work items (with type/state/title), and comment threads. Use `--json=false` for human-readable text output. `--max-threads N` limits the number of comment threads shown. Add `--git-diff` to fetch and display unified diffs of changed files in the pull request.

Post a comment on a pull request:

```bash
./tfs pr comment "https://dev.azure.com/your-org/YourProject/_git/your-repo/pullrequest/42" \
  --content "Looks good — approved" --json=false
```

Post a long comment from a file:

```bash
./tfs pr comment 42 --repository "your-repo" --content-file review-report.md
```

Post a comment from stdin:

```bash
echo "Please fix the null check in service.java" | ./tfs pr comment 42 --repository "your-repo" --content -
```

The `pr comment` command creates a new comment thread on the pull request. Content can be provided via `--content "text"`, `--content -` (stdin), or `--content-file <path>`. Optional `--status` sets the thread status (active, byDesign, resolved, closed, wontFix, unknown; default: active).

Show details and children:

```bash
./tfs show 123
```

Search:

```bash
./tfs search --query "report generation" --top 20
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
