package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"tfs-cli/internal/api"
	"tfs-cli/internal/config"
	"tfs-cli/internal/diff"
	"tfs-cli/internal/errs"
	"tfs-cli/internal/output"
)

const (
	maxBatchSize = 200
)

type stringFlag struct {
	value string
	set   bool
}

func (s *stringFlag) String() string {
	return s.value
}

func (s *stringFlag) Set(val string) error {
	s.value = val
	s.set = true
	return nil
}

type stringSliceFlag struct {
	values []string
}

func (s *stringSliceFlag) String() string {
	return strings.Join(s.values, ",")
}

func (s *stringSliceFlag) Set(val string) error {
	s.values = append(s.values, val)
	return nil
}

type globalFlags struct {
	baseURL  stringFlag
	project  stringFlag
	pat      stringFlag
	json     bool
	verbose  bool
	insecure bool
}

type commandContext struct {
	cfg      config.Config
	jsonMode bool
	verbose  bool
	insecure bool
	stdout   io.Writer
	stderr   io.Writer
	project  string
	baseURL  string
	pat      string
}

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "wiql":
		return runWiql(args[1:], stdout, stderr)
	case "view":
		return runView(args[1:], stdout, stderr)
	case "update":
		return runUpdate(args[1:], stdout, stderr)
	case "create":
		return runCreate(args[1:], stdout, stderr)
	case "delete":
		return runDelete(args[1:], stdout, stderr)
	case "search":
		return runSearch(args[1:], stdout, stderr)
	case "my":
		return runMy(args[1:], stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "pr":
		return runPR(args[1:], stdout, stderr)
	case "types":
		return runTypes(args[1:], stdout, stderr)
	case "whoami":
		return runWhoami(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	default:
		output.WriteError(stderr, errs.New("unknown_command", "unknown command", args[0]), true)
		return 1
	}
}

func runWiql(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("wiql", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	var top int
	fs.IntVar(&top, "top", 0, "Maximum number of results")
	queryArg, rest := splitPositional(args, wiqlValueFlags())
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if queryArg == "" {
		output.WriteError(stderr, errs.New("invalid_args", "WIQL query is required", nil), flags.json)
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}
	query := queryArg
	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	resp, err := client.Wiql(context.Background(), query, top)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	ids := collectIDs(resp)
	items, err := fetchWorkItems(context.Background(), client, ids)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	return renderList(ctx, items)
}

func runSearch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	var top int
	queryFlag := fs.String("query", "", "Search text")
	fs.IntVar(&top, "top", 0, "Maximum number of results")
	queryArg, rest := splitPositional(args, searchValueFlags())
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	query := *queryFlag
	if query == "" && queryArg != "" {
		query = queryArg
	}
	if query == "" {
		output.WriteError(stderr, errs.New("invalid_args", "search query is required", nil), flags.json)
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}
	query = wiqlSearchQuery(query)
	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	resp, err := client.Wiql(context.Background(), query, top)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	ids := collectIDs(resp)
	items, err := fetchWorkItems(context.Background(), client, ids)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	return renderList(ctx, items)
}

func runMy(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("my", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	var top int
	typeFilter := fs.String("type", "", "Work item type to include")
	allTypes := fs.Bool("all-types", true, "Do not filter by work item type")
	excludeState := fs.String("exclude-state", "", "Exclude items with this state (overrides default state filter)")
	allStates := fs.Bool("all-states", false, "Do not filter by state")
	fs.IntVar(&top, "top", 0, "Maximum number of results")
	jsonExplicit := flagProvided(args, "json")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if !jsonExplicit {
		ctx.jsonMode = false
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}
	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	allTypesEffective := *allTypes
	if strings.TrimSpace(*typeFilter) != "" {
		allTypesEffective = false
	}
	resp, err := client.Wiql(context.Background(), myWiqlQuery(*typeFilter, allTypesEffective, *excludeState, *allStates), top)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	ids := collectIDs(resp)
	items, err := fetchWorkItems(context.Background(), client, ids)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	return renderList(ctx, items)
}

func runView(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("view", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	var fieldsCSV string
	var expand string
	fs.StringVar(&fieldsCSV, "fields", "", "Comma-separated list of fields")
	fs.StringVar(&expand, "expand", "none", "Expand: none, relations, all")
	idArg, rest := splitPositional(args, viewValueFlags())
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if idArg == "" {
		output.WriteError(stderr, errs.New("invalid_args", "work item id is required", nil), flags.json)
		return 1
	}
	id, err := strconv.Atoi(idArg)
	if err != nil {
		output.WriteError(stderr, errs.New("invalid_args", "work item id must be a number", nil), flags.json)
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}
	fields := splitCSV(fieldsCSV)
	expandValue, err := mapExpand(expand)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	wi, err := client.GetWorkItem(context.Background(), id, fields, expandValue)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	return renderWorkItem(ctx, wi)
}

func runShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	childrenRel := fs.String("children-rel", "System.LinkTypes.Hierarchy-Forward", "Relation type used for children")
	maxChildren := fs.Int("max-children", 20, "Maximum number of children to show")
	idArg, rest := splitPositional(args, showValueFlags())
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if idArg == "" {
		output.WriteError(stderr, errs.New("invalid_args", "work item id is required", nil), flags.json)
		return 1
	}
	id, err := strconv.Atoi(idArg)
	if err != nil {
		output.WriteError(stderr, errs.New("invalid_args", "work item id must be a number", nil), flags.json)
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if !flagProvided(args, "json") {
		ctx.jsonMode = false
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}
	fields := []string{
		"System.Title",
		"System.Description",
		"System.AssignedTo",
		"System.Tags",
		"System.WorkItemType",
		"System.State",
		"System.History",
	}
	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	wi, err := client.GetWorkItem(context.Background(), id, fields, "None")
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	relWI, err := client.GetWorkItem(context.Background(), id, nil, "Relations")
	if err == nil {
		wi.Relations = relWI.Relations
	}
	normalized := output.NormalizeWorkItem(wi)
	childrenIDs := extractRelationIDs(wi.Relations, *childrenRel)
	if *maxChildren > 0 && len(childrenIDs) > *maxChildren {
		childrenIDs = childrenIDs[:*maxChildren]
	}
	children := []output.WorkItem{}
	if len(childrenIDs) > 0 {
		childItems, err := fetchWorkItems(context.Background(), client, childrenIDs)
		if err != nil {
			output.WriteError(stderr, err, ctx.jsonMode)
			return 1
		}
		children = childItems
	}
	if ctx.jsonMode {
		payload := map[string]interface{}{
			"workItem": normalized,
			"children": children,
			"raw":      wi,
		}
		if err := output.PrintJSON(ctx.stdout, payload); err != nil {
			output.WriteError(ctx.stderr, err, ctx.jsonMode)
			return 1
		}
		return 0
	}
	printWorkItemDetails(ctx.stdout, normalized, wi.Fields, children)
	return 0
}

func runUpdate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	sets := stringSliceFlag{}
	fs.Var(&sets, "set", "Field=Value (repeatable)")
	comment := fs.String("add-comment", "", "Add comment to System.History")
	parent := fs.Int("parent", 0, "Parent work item ID (reparent)")
	parentRel := fs.String("parent-rel", "System.LinkTypes.Hierarchy-Reverse", "Parent relation type")
	yes := fs.Bool("yes", false, "Confirm bulk updates (>5 fields)")
	idArg, rest := splitPositional(args, updateValueFlags())
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if idArg == "" {
		output.WriteError(stderr, errs.New("invalid_args", "work item id is required", nil), flags.json)
		return 1
	}
	id, err := strconv.Atoi(idArg)
	if err != nil {
		output.WriteError(stderr, errs.New("invalid_args", "work item id must be a number", nil), flags.json)
		return 1
	}
	if len(sets.values) == 0 && *comment == "" && *parent == 0 {
		output.WriteError(stderr, errs.New("invalid_args", "at least one --set, --add-comment, or --parent is required", nil), flags.json)
		return 1
	}
	if len(sets.values) > 5 && !*yes {
		output.WriteError(stderr, errs.New("confirmation_required", "more than 5 fields updated; use --yes to proceed", nil), flags.json)
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}

	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	patch, err := buildPatch(sets.values, *comment)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	parentPatch, err := buildParentPatch(context.Background(), client, id, *parent, *parentRel)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	patch = append(patch, parentPatch...)

	wi, err := client.UpdateWorkItem(context.Background(), id, patch)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	return renderWorkItem(ctx, wi)
}

func runDelete(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	yes := fs.Bool("yes", false, "Confirm deletion")
	destroy := fs.Bool("destroy", false, "Permanently destroy instead of moving to recycle bin; requires TFS destroy permission")
	idArg, rest := splitPositional(args, deleteValueFlags())
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if idArg == "" {
		output.WriteError(stderr, errs.New("invalid_args", "work item id is required", nil), flags.json)
		return 1
	}
	id, err := strconv.Atoi(idArg)
	if err != nil || id <= 0 {
		output.WriteError(stderr, errs.New("invalid_args", "work item id must be a positive number", nil), flags.json)
		return 1
	}
	if !*yes {
		output.WriteError(stderr, errs.New("confirmation_required", "delete is destructive; use --yes to proceed", id), flags.json)
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}
	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	deleted, err := client.DeleteWorkItem(context.Background(), id, *destroy)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	return renderDeletedWorkItem(ctx, id, *destroy, deleted)
}

func runCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	wiType := fs.String("type", "", "Work item type")
	title := fs.String("title", "", "Title")
	assigned := fs.String("assigned-to", "", "Assigned owner (defaults to PAT owner)")
	parent := fs.Int("parent", 0, "Parent work item ID")
	parentRel := fs.String("parent-rel", "System.LinkTypes.Hierarchy-Reverse", "Parent relation type")
	sets := stringSliceFlag{}
	fs.Var(&sets, "set", "Field=Value (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *wiType == "" || *title == "" {
		output.WriteError(stderr, errs.New("invalid_args", "--type and --title are required", nil), flags.json)
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}

	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	patch, err := buildCreatePatch(context.Background(), client, *title, *assigned, sets.values, *parent, *parentRel)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	wi, err := client.CreateWorkItem(context.Background(), *wiType, patch)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	return renderWorkItem(ctx, wi)
}

func runPR(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		output.WriteError(stderr, errs.New("invalid_args", "pr subcommand is required", nil), true)
		return 1
	}
	switch args[0] {
	case "create":
		return runPRCreate(args[1:], stdout, stderr)
	case "show":
		return runPRShow(args[1:], stdout, stderr)
	case "comment":
		return runPRComment(args[1:], stdout, stderr)
	default:
		output.WriteError(stderr, errs.New("unknown_command", "unknown pr subcommand", args[0]), true)
		return 1
	}
}

func runPRCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pr create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	repository := fs.String("repository", "", "Repository name or ID")
	source := fs.String("source", "", "Source branch")
	target := fs.String("target", "", "Target branch")
	title := fs.String("title", "", "Pull request title")
	description := fs.String("description", "", "Pull request description")
	draft := fs.Bool("draft", false, "Create as draft")
	autoComplete := fs.Bool("auto-complete", false, "Enable auto-complete after PR creation")
	var workItems stringSliceFlag
	fs.Var(&workItems, "work-item", "Work item ID to link to the PR (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if strings.TrimSpace(*repository) == "" || strings.TrimSpace(*source) == "" ||
		strings.TrimSpace(*target) == "" || strings.TrimSpace(*title) == "" {
		output.WriteError(stderr, errs.New("invalid_args",
			"--repository, --source, --target, and --title are required", nil), flags.json)
		return 1
	}

	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}

	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	workItemIDs, err := parsePositiveIDs(workItems.values, "work item")
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	req := api.CreatePullRequestRequest{
		SourceRefName: normalizeGitRef(*source),
		TargetRefName: normalizeGitRef(*target),
		Title:         strings.TrimSpace(*title),
		Description:   strings.TrimSpace(*description),
		IsDraft:       *draft,
	}
	for _, id := range workItemIDs {
		req.WorkItemRefs = append(req.WorkItemRefs, api.ResourceRef{
			ID:  strconv.Itoa(id),
			URL: client.WorkItemURL(id),
		})
	}

	pr, err := client.CreatePullRequest(context.Background(), strings.TrimSpace(*repository), req)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	if *autoComplete {
		autoCompleteIdentity, err := resolveAutoCompleteIdentity(context.Background(), client)
		if err != nil {
			output.WriteError(stderr, err, ctx.jsonMode)
			return 1
		}
		pr, err = client.UpdatePullRequest(context.Background(), strings.TrimSpace(*repository), pr.PullRequestID, api.UpdatePullRequestRequest{
			AutoCompleteSetBy: autoCompleteIdentity,
		})
		if err != nil {
			output.WriteError(stderr, err, ctx.jsonMode)
			return 1
		}
	}
	return renderPullRequest(ctx, pr)
}

func runPRShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pr show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	repository := fs.String("repository", "", "Repository name or ID (required when <id> is not a URL)")
	maxThreads := fs.Int("max-threads", 0, "Maximum number of comment threads to show (0 = all)")
	gitDiff := fs.Bool("git-diff", false, "Show git diff of pull request changes")
	arg, rest := splitPositional(args, prShowValueFlags())
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if arg == "" {
		output.WriteError(stderr, errs.New("invalid_args", "pull request URL or ID is required", nil), flags.json)
		return 1
	}

	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}

	var repositoryName string
	var prID int
	var baseURL string
	var project string

	if isURL(arg) {
		locator, err := parsePullRequestURL(arg)
		if err != nil {
			output.WriteError(stderr, err, flags.json)
			return 1
		}
		baseURL = locator.BaseURL
		project = locator.Project
		repositoryName = locator.Repository
		prID = locator.PullRequestID
	} else {
		id, err := strconv.Atoi(arg)
		if err != nil || id <= 0 {
			output.WriteError(stderr, errs.New("invalid_args", "pull request id must be a positive number or a URL", arg), flags.json)
			return 1
		}
		prID = id
		repositoryName = strings.TrimSpace(*repository)
		baseURL = ctx.baseURL
		project = ctx.project
	}

	if repositoryName == "" {
		output.WriteError(stderr, errs.New("invalid_args", "repository is required (use --repository or a full URL)", nil), flags.json)
		return 1
	}
	if project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}

	client, err := api.NewClient(baseURL, project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	pr, err := client.GetPullRequest(context.Background(), repositoryName, prID)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	threads, threadErr := client.GetPullRequestThreads(context.Background(), repositoryName, prID)
	if threadErr != nil {
		if ctx.verbose {
			fmt.Fprintf(stderr, "warning: could not fetch threads: %v\n", threadErr)
		}
		threads = nil
	}
	if *maxThreads > 0 && len(threads) > *maxThreads {
		threads = threads[:*maxThreads]
	}

	workItemRefs := pr.WorkItemRefs
	if len(workItemRefs) == 0 {
		wiRefs, wiErr := client.GetPullRequestWorkItems(context.Background(), repositoryName, prID)
		if wiErr != nil {
			if ctx.verbose {
				fmt.Fprintf(stderr, "warning: could not fetch work items: %v\n", wiErr)
			}
		} else {
			workItemRefs = wiRefs
		}
	}

	workItemIDs := make([]int, 0, len(workItemRefs))
	for _, ref := range workItemRefs {
		if id, err := strconv.Atoi(ref.ID); err == nil && id > 0 {
			workItemIDs = append(workItemIDs, id)
		}
	}
	workItems, err := fetchWorkItems(context.Background(), client, workItemIDs)
	if err != nil {
		if ctx.verbose {
			fmt.Fprintf(stderr, "warning: could not fetch work item details: %v\n", err)
		}
		workItems = nil
	}

	var fileDiffs []FileDiff
	if *gitDiff {
		fileDiffs, err = fetchPullRequestDiffs(context.Background(), client, repositoryName, pr, ctx.verbose, stderr)
		if err != nil && ctx.verbose {
			fmt.Fprintf(stderr, "warning: could not fetch git diff: %v\n", err)
		}
	}

	return renderPullRequestDetails(ctx, pr, workItems, threads, fileDiffs, *gitDiff)
}

func runPRComment(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pr comment", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	repository := fs.String("repository", "", "Repository name or ID (required when <id> is not a URL)")
	content := fs.String("content", "", "Comment content (use '-' for stdin)")
	contentFile := fs.String("content-file", "", "Read comment content from file")
	status := fs.String("status", "active", "Thread status: active, byDesign, resolved, closed, wontFix, unknown")
	arg, rest := splitPositional(args, prCommentValueFlags())
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	if arg == "" {
		output.WriteError(stderr, errs.New("invalid_args", "pull request URL or ID is required", nil), flags.json)
		return 1
	}

	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}

	var repositoryName string
	var prID int
	var baseURL string
	var project string

	if isURL(arg) {
		locator, err := parsePullRequestURL(arg)
		if err != nil {
			output.WriteError(stderr, err, flags.json)
			return 1
		}
		baseURL = locator.BaseURL
		project = locator.Project
		repositoryName = locator.Repository
		prID = locator.PullRequestID
	} else {
		id, err := strconv.Atoi(arg)
		if err != nil || id <= 0 {
			output.WriteError(stderr, errs.New("invalid_args", "pull request id must be a positive number or a URL", arg), flags.json)
			return 1
		}
		prID = id
		repositoryName = strings.TrimSpace(*repository)
		baseURL = ctx.baseURL
		project = ctx.project
	}

	if repositoryName == "" {
		output.WriteError(stderr, errs.New("invalid_args", "repository is required (use --repository or a full URL)", nil), flags.json)
		return 1
	}
	if project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}

	var commentText string
	if *contentFile != "" {
		data, err := os.ReadFile(*contentFile)
		if err != nil {
			output.WriteError(stderr, errs.New("read_error", "could not read content file", err.Error()), flags.json)
			return 1
		}
		commentText = string(data)
	} else if *content == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			output.WriteError(stderr, errs.New("read_error", "could not read stdin", err.Error()), flags.json)
			return 1
		}
		commentText = string(data)
	} else if *content != "" {
		commentText = *content
	} else {
		output.WriteError(stderr, errs.New("invalid_args", "comment content is required (use --content, --content -, or --content-file)", nil), flags.json)
		return 1
	}

	commentText = strings.TrimSpace(commentText)
	if commentText == "" {
		output.WriteError(stderr, errs.New("invalid_args", "comment content is empty", nil), flags.json)
		return 1
	}

	statusCode, err := mapThreadStatus(*status)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}

	client, err := api.NewClient(baseURL, project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	thread, err := client.CreatePullRequestThread(context.Background(), repositoryName, prID, api.CreatePullRequestThreadRequest{
		Comments: []api.CreatePullRequestComment{
			{
				Content:     commentText,
				CommentType: 1,
			},
		},
		Status: statusCode,
	})
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}

	return renderPullRequestComment(ctx, thread)
}

func renderPullRequestComment(ctx commandContext, thread api.GitPullRequestThread) int {
	if ctx.jsonMode {
		if err := output.PrintJSON(ctx.stdout, thread); err != nil {
			output.WriteError(ctx.stderr, err, ctx.jsonMode)
			return 1
		}
		return 0
	}
	fmt.Fprintf(ctx.stdout, "ThreadID: %d\n", thread.ID)
	if thread.Status != "" {
		fmt.Fprintf(ctx.stdout, "Status: %s\n", thread.Status)
	}
	for _, comment := range thread.Comments {
		if comment.IsDeleted {
			continue
		}
		author := identityDisplayName(comment.Author)
		date := comment.PublishedDate
		if date == "" {
			date = comment.LastUpdatedDate
		}
		prefix := author
		if date != "" {
			prefix = fmt.Sprintf("%s (%s)", author, date)
		}
		fmt.Fprintf(ctx.stdout, "Comment: %s\n", prefix)
		fmt.Fprintln(ctx.stdout, comment.Content)
	}
	return 0
}

func mapThreadStatus(value string) (int, error) {
	switch strings.ToLower(value) {
	case "active", "":
		return 1, nil
	case "bydesign":
		return 2, nil
	case "resolved":
		return 3, nil
	case "closed":
		return 4, nil
	case "wontfix":
		return 5, nil
	case "unknown":
		return 6, nil
	default:
		return 0, errs.New("invalid_args", "status must be one of: active, byDesign, resolved, closed, wontFix, unknown", value)
	}
}

func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runConfigView([]string{}, stdout, stderr)
	}
	switch args[0] {
	case "view":
		return runConfigView(args[1:], stdout, stderr)
	case "set":
		return runConfigSet(args[1:], stdout, stderr)
	default:
		output.WriteError(stderr, errs.New("unknown_command", "unknown config subcommand", args[0]), true)
		return 1
	}
}

func runTypes(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("types", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	if ctx.project == "" {
		output.WriteError(stderr, errs.New("config_missing", "project is required", nil), flags.json)
		return 1
	}
	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	types, err := client.ListWorkItemTypes(context.Background())
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	if ctx.jsonMode {
		payload := make([]map[string]interface{}, 0, len(types))
		for _, item := range types {
			payload = append(payload, map[string]interface{}{
				"name":          item.Name,
				"referenceName": item.ReferenceName,
				"isDisabled":    item.IsDisabled,
			})
		}
		if err := output.PrintJSON(ctx.stdout, payload); err != nil {
			output.WriteError(ctx.stderr, err, ctx.jsonMode)
			return 1
		}
		return 0
	}
	printTypeTable(ctx.stdout, types)
	return 0
}

func runWhoami(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("whoami", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := globalFlags{}
	addGlobalFlags(fs, &flags)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	ctx, err := buildContext(flags, stdout, stderr)
	if err != nil {
		output.WriteError(stderr, err, flags.json)
		return 1
	}
	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	profile, err := client.ProfileMe(context.Background())
	if err == nil {
		assigned := ""
		if profile.EmailAddress != "" && profile.DisplayName != "" {
			assigned = fmt.Sprintf("%s<%s>", profile.DisplayName, profile.EmailAddress)
		} else if profile.EmailAddress != "" {
			assigned = profile.EmailAddress
		} else if profile.DisplayName != "" {
			assigned = profile.DisplayName
		}
		if ctx.jsonMode {
			payload := map[string]interface{}{
				"displayName": profile.DisplayName,
				"email":       profile.EmailAddress,
				"id":          profile.ID,
				"assignedTo":  assigned,
				"source":      "profile",
			}
			if err := output.PrintJSON(ctx.stdout, payload); err != nil {
				output.WriteError(ctx.stderr, err, ctx.jsonMode)
				return 1
			}
			return 0
		}
		fmt.Fprintf(ctx.stdout, "DisplayName: %s\n", profile.DisplayName)
		fmt.Fprintf(ctx.stdout, "Email: %s\n", profile.EmailAddress)
		fmt.Fprintf(ctx.stdout, "ID: %s\n", profile.ID)
		fmt.Fprintf(ctx.stdout, "AssignedTo: %s\n", assigned)
		fmt.Fprintf(ctx.stdout, "Source: profile\n")
		return 0
	}

	identity, headerErr := client.WhoamiFromHeaders(context.Background())
	if headerErr != nil {
		output.WriteError(stderr, headerErr, ctx.jsonMode)
		return 1
	}
	var resolved *api.Identity
	if identity.ID != "" {
		resolved, _ = client.ResolveIdentityByID(context.Background(), identity.ID)
	}
	if ctx.jsonMode {
		assignedValue := identity.UniqueName
		if resolved != nil {
			assignedValue = fmt.Sprintf("{\"id\":\"%s\",\"displayName\":\"%s\",\"uniqueName\":\"%s\"}", resolved.ID, resolved.ProviderDisplayName, identityUniqueName(*resolved, identity.UniqueName))
		} else if identity.ID != "" {
			assignedValue = fmt.Sprintf("{\"id\":\"%s\",\"uniqueName\":\"%s\"}", identity.ID, identity.UniqueName)
		}
		payload := map[string]interface{}{
			"id":         identity.ID,
			"uniqueName": identity.UniqueName,
			"assignedTo": assignedValue,
			"source":     "headers",
		}
		if err := output.PrintJSON(ctx.stdout, payload); err != nil {
			output.WriteError(ctx.stderr, err, ctx.jsonMode)
			return 1
		}
		return 0
	}
	fmt.Fprintf(ctx.stdout, "ID: %s\n", identity.ID)
	fmt.Fprintf(ctx.stdout, "UniqueName: %s\n", identity.UniqueName)
	if resolved != nil {
		fmt.Fprintf(ctx.stdout, "AssignedTo: {\"id\":\"%s\",\"displayName\":\"%s\",\"uniqueName\":\"%s\"}\n", resolved.ID, resolved.ProviderDisplayName, identityUniqueName(*resolved, identity.UniqueName))
	} else if identity.ID != "" {
		fmt.Fprintf(ctx.stdout, "AssignedTo: {\"id\":\"%s\",\"uniqueName\":\"%s\"}\n", identity.ID, identity.UniqueName)
	} else {
		fmt.Fprintf(ctx.stdout, "AssignedTo: %s\n", identity.UniqueName)
	}
	fmt.Fprintf(ctx.stdout, "Source: headers\n")
	return 0
}

func runConfigView(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config view", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonMode := fs.Bool("json", true, "Output JSON (set --json=false for text)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	cfg, err := config.Load("")
	if err != nil {
		output.WriteError(stderr, err, *jsonMode)
		return 1
	}
	if *jsonMode {
		if err := output.PrintJSON(stdout, cfg.Redacted()); err != nil {
			output.WriteError(stderr, err, *jsonMode)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "BaseURL: %s\nProject: %s\nPAT: %s\n", cfg.BaseURL, cfg.Project, cfg.Redacted().PAT)
	return 0
}

func runConfigSet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config set", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonMode := fs.Bool("json", true, "Output JSON (set --json=false for text)")
	baseURL := stringFlag{}
	project := stringFlag{}
	pat := stringFlag{}
	fs.Var(&baseURL, "base-url", "Base URL")
	fs.Var(&project, "project", "Default project")
	fs.Var(&pat, "pat", "PAT token")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if !baseURL.set && !project.set && !pat.set {
		output.WriteError(stderr, errs.New("invalid_args", "at least one of --base-url, --project, or --pat is required", nil), *jsonMode)
		return 1
	}
	cfg, err := config.Load("")
	if err != nil {
		output.WriteError(stderr, err, *jsonMode)
		return 1
	}
	if baseURL.set {
		cfg.BaseURL = baseURL.value
	}
	if project.set {
		cfg.Project = project.value
	}
	if pat.set {
		cfg.PAT = pat.value
	}
	if err := config.Save("", cfg); err != nil {
		output.WriteError(stderr, err, *jsonMode)
		return 1
	}
	if *jsonMode {
		if err := output.PrintJSON(stdout, cfg.Redacted()); err != nil {
			output.WriteError(stderr, err, *jsonMode)
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, "Config updated")
	return 0
}

func addGlobalFlags(fs *flag.FlagSet, flags *globalFlags) {
	fs.Var(&flags.baseURL, "base-url", "Base URL")
	fs.Var(&flags.project, "project", "Project name")
	fs.Var(&flags.pat, "pat", "PAT token")
	fs.BoolVar(&flags.json, "json", true, "Output JSON (set --json=false for text)")
	fs.BoolVar(&flags.verbose, "verbose", false, "Verbose HTTP logging")
	fs.BoolVar(&flags.insecure, "insecure", false, "Skip TLS verification")
}

func buildContext(flags globalFlags, stdout, stderr io.Writer) (commandContext, error) {
	cfgFile, err := config.Load("")
	if err != nil {
		return commandContext{}, err
	}
	cfg := config.Merge(cfgFile, config.FromEnv())
	if flags.baseURL.set {
		cfg.BaseURL = flags.baseURL.value
	}
	if flags.project.set {
		cfg.Project = flags.project.value
	}
	if flags.pat.set {
		cfg.PAT = flags.pat.value
	}
	if cfg.BaseURL != "" && cfg.Project != "" {
		if normalized, ok := normalizeBaseURL(cfg.BaseURL, cfg.Project); ok {
			cfg.BaseURL = normalized
		}
	}
	return commandContext{
		cfg:      cfg,
		jsonMode: flags.json,
		verbose:  flags.verbose,
		insecure: flags.insecure,
		stdout:   stdout,
		stderr:   stderr,
		project:  cfg.Project,
		baseURL:  cfg.BaseURL,
		pat:      cfg.PAT,
	}, nil
}

func renderList(ctx commandContext, items []output.WorkItem) int {
	if ctx.jsonMode {
		if err := output.PrintJSON(ctx.stdout, items); err != nil {
			output.WriteError(ctx.stderr, err, ctx.jsonMode)
			return 1
		}
		return 0
	}
	output.PrintTable(ctx.stdout, items)
	return 0
}

func renderWorkItem(ctx commandContext, wi api.WorkItem) int {
	normalized := output.NormalizeWorkItem(wi)
	if ctx.jsonMode {
		payload := map[string]interface{}{
			"workItem": normalized,
			"raw":      wi,
		}
		if err := output.PrintJSON(ctx.stdout, payload); err != nil {
			output.WriteError(ctx.stderr, err, ctx.jsonMode)
			return 1
		}
		return 0
	}
	fmt.Fprintf(ctx.stdout, "ID: %d\n", normalized.ID)
	fmt.Fprintf(ctx.stdout, "Type: %s\n", stringValue(normalized.Type))
	fmt.Fprintf(ctx.stdout, "State: %s\n", stringValue(normalized.State))
	fmt.Fprintf(ctx.stdout, "Title: %s\n", stringValue(normalized.Title))
	fmt.Fprintf(ctx.stdout, "AssignedTo: %s\n", stringValue(normalized.AssignedTo))
	fmt.Fprintf(ctx.stdout, "AreaPath: %s\n", stringValue(normalized.AreaPath))
	fmt.Fprintf(ctx.stdout, "IterationPath: %s\n", stringValue(normalized.IterationPath))
	fmt.Fprintf(ctx.stdout, "Tags: %s\n", stringValue(normalized.Tags))
	fmt.Fprintf(ctx.stdout, "URL: %s\n", stringValue(normalized.URL))
	return 0
}

func renderDeletedWorkItem(ctx commandContext, id int, destroy bool, raw map[string]interface{}) int {
	if ctx.jsonMode {
		payload := map[string]interface{}{
			"id":      id,
			"deleted": true,
			"destroy": destroy,
			"raw":     raw,
		}
		if err := output.PrintJSON(ctx.stdout, payload); err != nil {
			output.WriteError(ctx.stderr, err, ctx.jsonMode)
			return 1
		}
		return 0
	}
	if destroy {
		fmt.Fprintf(ctx.stdout, "Permanently deleted work item %d\n", id)
		return 0
	}
	fmt.Fprintf(ctx.stdout, "Deleted work item %d\n", id)
	return 0
}

func renderPullRequest(ctx commandContext, pr api.GitPullRequest) int {
	if ctx.jsonMode {
		payload := map[string]interface{}{
			"pullRequestId":   pr.PullRequestID,
			"status":          pr.Status,
			"title":           pr.Title,
			"description":     pr.Description,
			"repository":      pr.Repository.Name,
			"sourceRefName":   pr.SourceRefName,
			"targetRefName":   pr.TargetRefName,
			"isDraft":         pr.IsDraft,
			"hasAutoComplete": pr.AutoCompleteSetBy != nil && pr.AutoCompleteSetBy.ID != "",
			"workItemIds":     resourceRefIDs(pr.WorkItemRefs),
			"url":             pullRequestURL(pr),
			"apiUrl":          pr.URL,
			"raw":             pr,
		}
		if err := output.PrintJSON(ctx.stdout, payload); err != nil {
			output.WriteError(ctx.stderr, err, ctx.jsonMode)
			return 1
		}
		return 0
	}
	fmt.Fprintf(ctx.stdout, "PullRequestID: %d\n", pr.PullRequestID)
	fmt.Fprintf(ctx.stdout, "Status: %s\n", pr.Status)
	fmt.Fprintf(ctx.stdout, "Title: %s\n", pr.Title)
	fmt.Fprintf(ctx.stdout, "Repository: %s\n", pr.Repository.Name)
	fmt.Fprintf(ctx.stdout, "Source: %s\n", pr.SourceRefName)
	fmt.Fprintf(ctx.stdout, "Target: %s\n", pr.TargetRefName)
	fmt.Fprintf(ctx.stdout, "IsDraft: %t\n", pr.IsDraft)
	fmt.Fprintf(ctx.stdout, "AutoComplete: %t\n", pr.AutoCompleteSetBy != nil && pr.AutoCompleteSetBy.ID != "")
	if ids := resourceRefIDs(pr.WorkItemRefs); len(ids) > 0 {
		fmt.Fprintf(ctx.stdout, "WorkItems: %s\n", strings.Join(ids, ", "))
	}
	fmt.Fprintf(ctx.stdout, "URL: %s\n", pullRequestURL(pr))
	return 0
}

func renderPullRequestDetails(ctx commandContext, pr api.GitPullRequest, workItems []output.WorkItem, threads []api.GitPullRequestThread, fileDiffs []FileDiff, showDiff bool) int {
	if ctx.jsonMode {
		payload := map[string]interface{}{
			"pullRequestId":   pr.PullRequestID,
			"status":          pr.Status,
			"title":           pr.Title,
			"description":     pr.Description,
			"repository":      pr.Repository.Name,
			"sourceRefName":   pr.SourceRefName,
			"targetRefName":   pr.TargetRefName,
			"isDraft":         pr.IsDraft,
			"creationDate":    pr.CreationDate,
			"createdBy":       identityDisplayName(pr.CreatedBy),
			"hasAutoComplete": pr.AutoCompleteSetBy != nil && pr.AutoCompleteSetBy.ID != "",
			"workItemIds":     resourceRefIDs(pr.WorkItemRefs),
			"workItems":       workItems,
			"threads":         threads,
			"url":             pullRequestURL(pr),
			"apiUrl":          pr.URL,
			"raw":             pr,
		}
		if showDiff {
			payload["gitDiff"] = fileDiffs
		}
		if err := output.PrintJSON(ctx.stdout, payload); err != nil {
			output.WriteError(ctx.stderr, err, ctx.jsonMode)
			return 1
		}
		return 0
	}

	fmt.Fprintf(ctx.stdout, "PullRequestID: %d\n", pr.PullRequestID)
	fmt.Fprintf(ctx.stdout, "Repository: %s\n", pr.Repository.Name)
	fmt.Fprintf(ctx.stdout, "Title: %s\n", pr.Title)
	fmt.Fprintf(ctx.stdout, "Status: %s\n", pr.Status)
	if author := identityDisplayName(pr.CreatedBy); author != "" {
		fmt.Fprintf(ctx.stdout, "Author: %s\n", author)
	}
	fmt.Fprintf(ctx.stdout, "Branches: %s -> %s\n", shortRef(pr.SourceRefName), shortRef(pr.TargetRefName))
	fmt.Fprintf(ctx.stdout, "IsDraft: %t\n", pr.IsDraft)
	if pr.CreationDate != "" {
		fmt.Fprintf(ctx.stdout, "Created: %s\n", pr.CreationDate)
	}
	fmt.Fprintln(ctx.stdout)

	if strings.TrimSpace(pr.Description) != "" {
		fmt.Fprintln(ctx.stdout, "Description:")
		fmt.Fprintln(ctx.stdout, pr.Description)
		fmt.Fprintln(ctx.stdout)
	}

	if len(workItems) > 0 {
		fmt.Fprintln(ctx.stdout, "Work Items:")
		output.PrintTable(ctx.stdout, workItems)
		fmt.Fprintln(ctx.stdout)
	} else {
		fmt.Fprintln(ctx.stdout, "Work Items: none")
		fmt.Fprintln(ctx.stdout)
	}

	activeThreads := 0
	for _, thread := range threads {
		if thread.IsDeleted {
			continue
		}
		activeThreads++
	}
	if activeThreads > 0 {
		fmt.Fprintln(ctx.stdout, "Comments:")
		for _, thread := range threads {
			if thread.IsDeleted {
				continue
			}
			threadLabel := fmt.Sprintf("Thread %d", thread.ID)
			if thread.Status != "" {
				threadLabel += " [" + thread.Status + "]"
			}
			fmt.Fprintf(ctx.stdout, "  %s\n", threadLabel)
			for _, comment := range thread.Comments {
				if comment.IsDeleted || strings.TrimSpace(comment.Content) == "" {
					continue
				}
				author := identityDisplayName(comment.Author)
				date := comment.PublishedDate
				if date == "" {
					date = comment.LastUpdatedDate
				}
				prefix := author
				if date != "" {
					prefix = fmt.Sprintf("%s (%s)", author, date)
				}
				fmt.Fprintf(ctx.stdout, "    %s: %s\n", prefix, comment.Content)
			}
		}
		fmt.Fprintln(ctx.stdout)
	} else {
		fmt.Fprintln(ctx.stdout, "Comments: none")
		fmt.Fprintln(ctx.stdout)
	}

	if showDiff {
		renderGitDiffText(ctx.stdout, fileDiffs)
	}

	fmt.Fprintf(ctx.stdout, "URL: %s\n", pullRequestURL(pr))
	return 0
}

func renderGitDiffText(w io.Writer, fileDiffs []FileDiff) {
	if len(fileDiffs) == 0 {
		fmt.Fprintln(w, "Git Diff: no changes")
		fmt.Fprintln(w)
		return
	}

	addCount, editCount, delCount := 0, 0, 0
	for _, fd := range fileDiffs {
		switch strings.ToLower(fd.ChangeType) {
		case "add":
			addCount++
		case "edit":
			editCount++
		case "delete":
			delCount++
		}
	}

	var summary []string
	if addCount > 0 {
		summary = append(summary, fmt.Sprintf("%d add", addCount))
	}
	if editCount > 0 {
		summary = append(summary, fmt.Sprintf("%d edit", editCount))
	}
	if delCount > 0 {
		summary = append(summary, fmt.Sprintf("%d delete", delCount))
	}
	summaryStr := ""
	if len(summary) > 0 {
		summaryStr = ", " + strings.Join(summary, ", ")
	}
	fmt.Fprintf(w, "Git Diff (%d files changed%s):\n", len(fileDiffs), summaryStr)

	for _, fd := range fileDiffs {
		changeType := fd.ChangeType
		if changeType == "" {
			changeType = "edit"
		}
		path := fd.Path
		if path == "" {
			path = "(unknown path)"
		}
		fmt.Fprintf(w, "\n%s %s\n", changeTypeLabel(changeType), path)
		if fd.Error != "" {
			fmt.Fprintf(w, "  (error: %s)\n", fd.Error)
		} else if strings.TrimSpace(fd.Diff) != "" {
			fmt.Fprint(w, fd.Diff)
			if !strings.HasSuffix(fd.Diff, "\n") {
				fmt.Fprintln(w)
			}
		}
	}
	fmt.Fprintln(w)
}

func changeTypeLabel(ct string) string {
	switch strings.ToLower(ct) {
	case "add":
		return "A"
	case "edit":
		return "M"
	case "delete":
		return "D"
	case "rename":
		return "R"
	default:
		return strings.ToUpper(ct[:1])
	}
}

type FileDiff struct {
	ChangeType string `json:"changeType"`
	Path       string `json:"path"`
	Diff       string `json:"diff,omitempty"`
	Error      string `json:"error,omitempty"`
}

func fetchPullRequestDiffs(ctx context.Context, client *api.Client, repository string, pr api.GitPullRequest, verbose bool, stderr io.Writer) ([]FileDiff, error) {
	iterations, err := client.GetPullRequestIterations(ctx, repository, pr.PullRequestID)
	if err != nil {
		return nil, err
	}
	if len(iterations) == 0 {
		return nil, errs.New("no_iterations", "pull request has no iterations", nil)
	}

	latest := iterations[0]
	for _, it := range iterations[1:] {
		if it.ID > latest.ID {
			latest = it
		}
	}

	changes, err := client.GetPullRequestIterationChanges(ctx, repository, pr.PullRequestID, latest.ID)
	if err != nil {
		return nil, err
	}

	baseVersion := ""
	targetVersion := ""
	baseVersionType := "commit"
	if pr.LastMergeTargetCommit != nil && pr.LastMergeTargetCommit.CommitID != "" {
		baseVersion = pr.LastMergeTargetCommit.CommitID
	} else if latest.TargetRefCommit != nil && latest.TargetRefCommit.CommitID != "" {
		baseVersion = latest.TargetRefCommit.CommitID
	} else {
		baseVersion = shortRef(pr.TargetRefName)
		baseVersionType = "branch"
	}
	if pr.LastMergeSourceCommit != nil && pr.LastMergeSourceCommit.CommitID != "" {
		targetVersion = pr.LastMergeSourceCommit.CommitID
	} else if latest.SourceRefCommit != nil && latest.SourceRefCommit.CommitID != "" {
		targetVersion = latest.SourceRefCommit.CommitID
	} else {
		targetVersion = shortRef(pr.SourceRefName)
	}

	fileDiffs := make([]FileDiff, 0, len(changes))
	for _, change := range changes {
		if change.Item.GitObjectType == "tree" {
			continue
		}
		path := change.Item.Path
		changeType := strings.ToLower(change.ChangeType)
		if changeType == "" {
			changeType = "edit"
		}

		fd := FileDiff{ChangeType: changeType, Path: path}

		var oldContent, newContent string
		var fetchErr error

		switch changeType {
		case "add":
			newContent, fetchErr = client.GetItemContent(ctx, repository, path, "commit", targetVersion)
			if fetchErr != nil {
				fd.Error = fmt.Sprintf("could not fetch new content: %v", fetchErr)
			}
		case "delete":
			oldContent, fetchErr = client.GetItemContent(ctx, repository, path, baseVersionType, baseVersion)
			if fetchErr != nil {
				fd.Error = fmt.Sprintf("could not fetch old content: %v", fetchErr)
			}
		default:
			oldContent, fetchErr = client.GetItemContent(ctx, repository, path, baseVersionType, baseVersion)
			if fetchErr != nil && verbose {
				fmt.Fprintf(stderr, "  warning: could not fetch old content for %s: %v\n", path, fetchErr)
			}
			newContent, fetchErr = client.GetItemContent(ctx, repository, path, "commit", targetVersion)
			if fetchErr != nil && verbose {
				fmt.Fprintf(stderr, "  warning: could not fetch new content for %s: %v\n", path, fetchErr)
			}
		}

		if fd.Error == "" {
			fd.Diff = diff.UnifiedDiff(oldContent, newContent)
		}

		fileDiffs = append(fileDiffs, fd)
	}

	return fileDiffs, nil
}

func parsePullRequestURL(rawURL string) (pullRequestLocator, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return pullRequestLocator{}, errs.New("invalid_url", "could not parse URL", err.Error())
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	gitIdx := -1
	for i, seg := range segments {
		if seg == "_git" {
			gitIdx = i
			break
		}
	}
	if gitIdx < 0 || gitIdx == 0 {
		return pullRequestLocator{}, errs.New("invalid_url", "URL must contain /<project>/_git/<repo>/pullrequest/<id>", rawURL)
	}
	project := segments[gitIdx-1]
	baseSegments := segments[:gitIdx-1]
	baseURL := parsed.Scheme + "://" + parsed.Host
	if len(baseSegments) > 0 {
		baseURL += "/" + strings.Join(baseSegments, "/")
	}

	if gitIdx+1 >= len(segments) {
		return pullRequestLocator{}, errs.New("invalid_url", "URL missing repository name after _git", rawURL)
	}
	repository := segments[gitIdx+1]

	prIdx := -1
	for i := gitIdx + 2; i < len(segments); i++ {
		if segments[i] == "pullrequest" || segments[i] == "pullrequests" {
			prIdx = i
			break
		}
	}
	if prIdx < 0 || prIdx+1 >= len(segments) {
		return pullRequestLocator{}, errs.New("invalid_url", "URL missing pullrequest/<id> segment", rawURL)
	}
	prID, err := strconv.Atoi(segments[prIdx+1])
	if err != nil || prID <= 0 {
		return pullRequestLocator{}, errs.New("invalid_url", "pull request id must be a positive number", segments[prIdx+1])
	}

	return pullRequestLocator{
		BaseURL:        baseURL,
		Project:        project,
		Repository:     repository,
		PullRequestID: prID,
	}, nil
}

type pullRequestLocator struct {
	BaseURL        string
	Project        string
	Repository     string
	PullRequestID int
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func shortRef(ref string) string {
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}
	return ref
}

func identityDisplayName(ref map[string]interface{}) string {
	if ref == nil {
		return ""
	}
	if dn, ok := ref["displayName"].(string); ok && dn != "" {
		return dn
	}
	if un, ok := ref["uniqueName"].(string); ok && un != "" {
		return un
	}
	return ""
}

func prShowValueFlags() map[string]bool {
	flags := wiqlValueFlags()
	flags["repository"] = true
	flags["max-threads"] = true
	return flags
}

func prCommentValueFlags() map[string]bool {
	flags := wiqlValueFlags()
	flags["repository"] = true
	flags["content"] = true
	flags["content-file"] = true
	flags["status"] = true
	return flags
}

func collectIDs(resp api.WiqlResponse) []int {
	seen := map[int]bool{}
	ids := []int{}
	for _, item := range resp.WorkItems {
		if !seen[item.ID] {
			ids = append(ids, item.ID)
			seen[item.ID] = true
		}
	}
	for _, link := range resp.WorkItemLinks {
		if link.Source.ID != 0 && !seen[link.Source.ID] {
			ids = append(ids, link.Source.ID)
			seen[link.Source.ID] = true
		}
		if link.Target.ID != 0 && !seen[link.Target.ID] {
			ids = append(ids, link.Target.ID)
			seen[link.Target.ID] = true
		}
	}
	return ids
}

func fetchWorkItems(ctx context.Context, client *api.Client, ids []int) ([]output.WorkItem, error) {
	if len(ids) == 0 {
		return []output.WorkItem{}, nil
	}
	fields := []string{
		"System.WorkItemType",
		"System.State",
		"System.Title",
		"System.AssignedTo",
		"System.AreaPath",
		"System.IterationPath",
		"System.Tags",
	}
	results := make([]output.WorkItem, 0, len(ids))
	for i := 0; i < len(ids); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		items, err := client.GetWorkItemsBatch(ctx, chunk, fields)
		if err != nil {
			return nil, err
		}
		ordered := orderWorkItems(chunk, items)
		results = append(results, ordered...)
	}
	return results, nil
}

func orderWorkItems(ids []int, items []api.WorkItem) []output.WorkItem {
	byID := make(map[int]api.WorkItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	ordered := make([]output.WorkItem, 0, len(ids))
	for _, id := range ids {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, output.NormalizeWorkItem(item))
		}
	}
	return ordered
}

func buildPatch(sets []string, comment string) ([]map[string]interface{}, error) {
	patch := []map[string]interface{}{}
	for _, set := range sets {
		field, value, err := parseAssignment(set)
		if err != nil {
			return nil, err
		}
		patch = append(patch, map[string]interface{}{
			"op":    "add",
			"path":  "/fields/" + field,
			"value": value,
		})
	}
	if comment != "" {
		// TODO: confirm System.History as the comment field for API 6.0.
		patch = append(patch, map[string]interface{}{
			"op":    "add",
			"path":  "/fields/System.History",
			"value": comment,
		})
	}
	return patch, nil
}

func buildCreatePatch(ctx context.Context, client *api.Client, title string, assigned string, sets []string, parentID int, parentRel string) ([]map[string]interface{}, error) {
	patch := []map[string]interface{}{}
	patch = append(patch, map[string]interface{}{
		"op":    "add",
		"path":  "/fields/System.Title",
		"value": title,
	})

	var assignedValue interface{} = ""
	assignedString := assigned
	remainingSets := []string{}
	for _, set := range sets {
		field, value, err := parseAssignment(set)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(field, "System.AssignedTo") {
			if assignedString == "" {
				assignedString = value
			}
			continue
		}
		remainingSets = append(remainingSets, set)
	}
	if assignedString != "" {
		assignedValue = assignedString
	}
	if assignedValue == "" {
		profile, err := client.ProfileMe(ctx)
		if err == nil {
			if profile.EmailAddress != "" && profile.DisplayName != "" {
				assignedValue = fmt.Sprintf("%s<%s>", profile.DisplayName, profile.EmailAddress)
			} else if profile.EmailAddress != "" {
				assignedValue = profile.EmailAddress
			} else if profile.DisplayName != "" {
				assignedValue = profile.DisplayName
			}
		} else {
			identity, headerErr := client.WhoamiFromHeaders(ctx)
			if headerErr == nil {
				if identity.ID != "" {
					resolved, resolveErr := client.ResolveIdentityByID(ctx, identity.ID)
					if resolveErr == nil && resolved != nil {
						assignedValue = identityRefValue(*resolved, identity.UniqueName)
					} else {
						assignedValue = identityRefFallback(identity)
					}
				} else if identity.UniqueName != "" {
					assignedValue = identity.UniqueName
				}
			}
			if assignedValue == "" {
				return nil, errs.New("assigned_to_required", "assigned-to is required and could not be resolved from PAT profile", err.Error())
			}
		}
	}
	if assignedValue == "" {
		return nil, errs.New("assigned_to_required", "assigned-to is required", nil)
	}
	patch = append(patch, map[string]interface{}{
		"op":    "add",
		"path":  "/fields/System.AssignedTo",
		"value": assignedValue,
	})

	if parentID > 0 {
		relationType := parentRel
		if relationType == "" {
			relationType = "System.LinkTypes.Hierarchy-Reverse"
		}
		// TODO: confirm link relation type and URL shape for parent in your API 6.0 docs.
		patch = append(patch, map[string]interface{}{
			"op":   "add",
			"path": "/relations/-",
			"value": map[string]interface{}{
				"rel": relationType,
				"url": client.WorkItemURL(parentID),
			},
		})
	}

	setPatch, err := buildPatch(remainingSets, "")
	if err != nil {
		return nil, err
	}
	patch = append(patch, setPatch...)
	return patch, nil
}

func buildParentPatch(ctx context.Context, client *api.Client, itemID int, parentID int, parentRel string) ([]map[string]interface{}, error) {
	if parentID == 0 {
		return nil, nil
	}
	if parentRel == "" {
		parentRel = "System.LinkTypes.Hierarchy-Reverse"
	}

	wi, err := client.GetWorkItem(ctx, itemID, nil, "relations")
	if err != nil {
		return nil, err
	}

	patch := []map[string]interface{}{}
	existingParentIndices := []int{}
	existingParentID := 0
	for i, raw := range wi.Relations {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		rel, _ := m["rel"].(string)
		if rel == parentRel {
			existingParentIndices = append(existingParentIndices, i)
			url, _ := m["url"].(string)
			if existingParentID == 0 {
				existingParentID = idFromURL(url)
			}
		}
	}

	if existingParentID == parentID && len(existingParentIndices) == 1 {
		return nil, nil
	}

	for i := len(existingParentIndices) - 1; i >= 0; i-- {
		idx := existingParentIndices[i]
		patch = append(patch, map[string]interface{}{
			"op":    "remove",
			"path":  fmt.Sprintf("/relations/%d", idx),
		})
	}

	patch = append(patch, map[string]interface{}{
		"op":   "add",
		"path": "/relations/-",
		"value": map[string]interface{}{
			"rel": parentRel,
			"url": client.WorkItemURL(parentID),
		},
	})

	return patch, nil
}

func parseAssignment(input string) (string, string, error) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return "", "", errs.New("invalid_args", "invalid --set format, expected Field=Value", input)
	}
	field := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if field == "" {
		return "", "", errs.New("invalid_args", "field name is required", input)
	}
	return field, value, nil
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func mapExpand(value string) (string, error) {
	switch strings.ToLower(value) {
	case "none", "":
		return "None", nil
	case "relations":
		return "Relations", nil
	case "all":
		return "All", nil
	default:
		return "", errs.New("invalid_args", "expand must be none, relations, or all", value)
	}
}

func normalizeGitRef(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "refs/") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "heads/") {
		return "refs/" + trimmed
	}
	return "refs/heads/" + trimmed
}

func pullRequestURL(pr api.GitPullRequest) string {
	if pr.Links != nil {
		if link, ok := pr.Links["web"]; ok && strings.TrimSpace(link.Href) != "" {
			return link.Href
		}
	}
	if strings.TrimSpace(pr.Repository.WebURL) != "" {
		return strings.TrimRight(pr.Repository.WebURL, "/") + "/pullrequest/" + strconv.Itoa(pr.PullRequestID)
	}
	if strings.TrimSpace(pr.Repository.RemoteURL) != "" {
		return strings.TrimRight(pr.Repository.RemoteURL, "/") + "/pullrequest/" + strconv.Itoa(pr.PullRequestID)
	}
	return pr.URL
}

func wiqlSearchQuery(text string) string {
	escaped := strings.ReplaceAll(text, "'", "''")
	return fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE ([System.Title] CONTAINS '%s' OR [System.Description] CONTAINS '%s') ORDER BY [System.ChangedDate] DESC", escaped, escaped)
}

func myWiqlQuery(typeFilter string, allTypes bool, excludeState string, allStates bool) string {
	conditions := []string{"[System.TeamProject] = @Project", "[System.AssignedTo] = @Me"}
	if !allTypes && strings.TrimSpace(typeFilter) != "" {
		conditions = append(conditions, fmt.Sprintf("[System.WorkItemType] = '%s'", escapeWiql(typeFilter)))
	}
	if !allStates && strings.TrimSpace(excludeState) != "" {
		conditions = append(conditions, fmt.Sprintf("[System.State] <> '%s'", escapeWiql(excludeState)))
	} else if !allStates {
		conditions = append(conditions, fmt.Sprintf("[System.State] IN (%s)", joinWiqlValues([]string{"Разработка", "Выполняется"})))
	}
	return fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE %s ORDER BY [System.ChangedDate] DESC", strings.Join(conditions, " AND "))
}

func escapeWiql(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func joinWiqlValues(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		quoted = append(quoted, fmt.Sprintf("'%s'", escapeWiql(trimmed)))
	}
	return strings.Join(quoted, ", ")
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func normalizeBaseURL(baseURL, project string) (string, bool) {
	if baseURL == "" || project == "" {
		return baseURL, false
	}
	trimmed := strings.TrimRight(baseURL, "/")
	lowerTrim := strings.ToLower(trimmed)
	lowerProject := strings.ToLower(project)
	if !strings.HasSuffix(lowerTrim, "/"+lowerProject) {
		return baseURL, false
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" {
		pathTrim := strings.TrimRight(parsed.Path, "/")
		parts := strings.Split(pathTrim, "/")
		if len(parts) > 0 && strings.EqualFold(parts[len(parts)-1], project) {
			parts = parts[:len(parts)-1]
			parsed.Path = strings.Join(parts, "/")
			if parsed.Path == "" {
				parsed.Path = "/"
			}
			return strings.TrimRight(parsed.String(), "/"), true
		}
	}
	return strings.TrimRight(trimmed[:len(trimmed)-len(project)-1], "/"), true
}

func flagProvided(args []string, name string) bool {
	long := "--" + name
	for _, arg := range args {
		if arg == long {
			return true
		}
		if strings.HasPrefix(arg, long+"=") {
			return true
		}
	}
	return false
}

func showValueFlags() map[string]bool {
	flags := wiqlValueFlags()
	flags["children-rel"] = true
	flags["max-children"] = true
	return flags
}

func parsePositiveIDs(raw []string, entityName string) ([]int, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	ids := make([]int, 0, len(raw))
	for _, item := range raw {
		value := strings.TrimSpace(item)
		if value == "" {
			return nil, errs.New("invalid_args", entityName+" id is required", nil)
		}
		id, err := strconv.Atoi(value)
		if err != nil {
			return nil, errs.New("invalid_args", entityName+" id must be a number", value)
		}
		if id <= 0 {
			return nil, errs.New("invalid_args", entityName+" id must be a positive number", value)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func resolveAutoCompleteIdentity(ctx context.Context, client *api.Client) (*api.IdentityRef, error) {
	identity, headerErr := client.WhoamiFromHeaders(ctx)
	if headerErr == nil && identity.ID != "" {
		ref := &api.IdentityRef{
			ID:         identity.ID,
			UniqueName: identity.UniqueName,
		}
		if resolved, err := client.ResolveIdentityByID(ctx, identity.ID); err == nil && resolved != nil {
			ref.DisplayName = resolved.ProviderDisplayName
			ref.UniqueName = identityUniqueName(*resolved, identity.UniqueName)
		}
		return ref, nil
	}

	profile, profileErr := client.ProfileMe(ctx)
	if profileErr == nil && profile.ID != "" {
		return &api.IdentityRef{
			ID:          profile.ID,
			DisplayName: profile.DisplayName,
			UniqueName:  profile.EmailAddress,
		}, nil
	}

	if headerErr != nil {
		return nil, errs.New("identity_not_found", "could not resolve current identity for auto-complete", headerErr.Error())
	}
	if profileErr != nil {
		return nil, errs.New("identity_not_found", "could not resolve current identity for auto-complete", profileErr.Error())
	}
	return nil, errs.New("identity_not_found", "could not resolve current identity for auto-complete", nil)
}

func resourceRefIDs(refs []api.ResourceRef) []string {
	if len(refs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref.ID) != "" {
			ids = append(ids, ref.ID)
		}
	}
	return ids
}

func identityRefValue(identity api.Identity, fallbackUnique string) map[string]interface{} {
	ref := map[string]interface{}{
		"id": identity.ID,
	}
	if identity.ProviderDisplayName != "" {
		ref["displayName"] = identity.ProviderDisplayName
	}
	if identity.SubjectDescriptor != "" {
		ref["descriptor"] = identity.SubjectDescriptor
	} else if identity.Descriptor != "" {
		ref["descriptor"] = identity.Descriptor
	}
	if unique := identityUniqueName(identity, fallbackUnique); unique != "" {
		ref["uniqueName"] = unique
	}
	return ref
}

func identityUniqueName(identity api.Identity, fallback string) string {
	domain := identityProperty(identity, "Domain")
	account := identityProperty(identity, "Account")
	if domain != "" && account != "" {
		return domain + `\\` + account
	}
	if value := identityProperty(identity, "Mail"); value != "" {
		return value
	}
	if value := identityProperty(identity, "Account"); value != "" {
		return value
	}
	if value := identityProperty(identity, "UniqueName"); value != "" {
		return value
	}
	return fallback
}

func identityProperty(identity api.Identity, key string) string {
	if identity.Properties == nil {
		return ""
	}
	raw, ok := identity.Properties[key]
	if !ok {
		return ""
	}
	if prop, ok := raw.(map[string]interface{}); ok {
		if val, ok := prop["$value"].(string); ok {
			return val
		}
	}
	if val, ok := raw.(string); ok {
		return val
	}
	return ""
}

func identityRefFallback(identity api.HeaderIdentity) interface{} {
	if identity.ID != "" {
		return map[string]interface{}{
			"id":         identity.ID,
			"uniqueName": identity.UniqueName,
		}
	}
	if identity.UniqueName != "" {
		return identity.UniqueName
	}
	return ""
}

func extractRelationIDs(relations []interface{}, relFilter string) []int {
	if len(relations) == 0 {
		return nil
	}
	ids := []int{}
	seen := map[int]bool{}
	for _, raw := range relations {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		rel, _ := m["rel"].(string)
		if relFilter != "" && rel != relFilter {
			continue
		}
		url, _ := m["url"].(string)
		id := idFromURL(url)
		if id == 0 || seen[id] {
			continue
		}
		ids = append(ids, id)
		seen[id] = true
	}
	return ids
}

func idFromURL(value string) int {
	if value == "" {
		return 0
	}
	parts := strings.Split(strings.TrimRight(value, "/"), "/")
	if len(parts) == 0 {
		return 0
	}
	id, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0
	}
	return id
}

func printWorkItemDetails(w io.Writer, wi output.WorkItem, fields map[string]interface{}, children []output.WorkItem) {
	fmt.Fprintf(w, "ID: %d\n", wi.ID)
	fmt.Fprintf(w, "Title: %s\n", stringValue(wi.Title))
	fmt.Fprintf(w, "Type: %s\n", stringValue(wi.Type))
	fmt.Fprintf(w, "State: %s\n", stringValue(wi.State))
	fmt.Fprintf(w, "AssignedTo: %s\n", stringValue(wi.AssignedTo))
	fmt.Fprintf(w, "Tags: %s\n", stringValue(wi.Tags))
	fmt.Fprintln(w, "")
	if desc, ok := fields["System.Description"].(string); ok && desc != "" {
		fmt.Fprintln(w, "Description:")
		fmt.Fprintln(w, desc)
		fmt.Fprintln(w, "")
	}
	if history, ok := fields["System.History"].(string); ok && history != "" {
		fmt.Fprintln(w, "Comment (latest):")
		fmt.Fprintln(w, history)
		fmt.Fprintln(w, "")
	}
	if len(children) == 0 {
		fmt.Fprintln(w, "Children: none")
		return
	}
	fmt.Fprintln(w, "Children:")
	output.PrintTable(w, children)
}

func splitPositional(args []string, valueFlags map[string]bool) (string, []string) {
	var positional string
	rest := make([]string, 0, len(args))
	skipNext := false
	for _, arg := range args {
		if skipNext {
			rest = append(rest, arg)
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			rest = append(rest, arg)
			name := strings.TrimLeft(arg, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				name = name[:eq]
			} else if valueFlags[name] {
				skipNext = true
			}
			continue
		}
		if positional == "" {
			positional = arg
			continue
		}
		rest = append(rest, arg)
	}
	return positional, rest
}

func wiqlValueFlags() map[string]bool {
	return map[string]bool{
		"top":      true,
		"base-url": true,
		"project":  true,
		"pat":      true,
	}
}

func searchValueFlags() map[string]bool {
	flags := wiqlValueFlags()
	flags["query"] = true
	return flags
}

func viewValueFlags() map[string]bool {
	flags := wiqlValueFlags()
	flags["fields"] = true
	flags["expand"] = true
	return flags
}

func updateValueFlags() map[string]bool {
	flags := wiqlValueFlags()
	flags["set"] = true
	flags["add-comment"] = true
	flags["parent"] = true
	flags["parent-rel"] = true
	flags["yes"] = false
	return flags
}

func deleteValueFlags() map[string]bool {
	flags := wiqlValueFlags()
	flags["yes"] = false
	flags["destroy"] = false
	return flags
}

func printTypeTable(w io.Writer, types []api.WorkItemType) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tREFERENCE\tDISABLED")
	for _, item := range types {
		disabled := "no"
		if item.IsDisabled {
			disabled = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", item.Name, item.ReferenceName, disabled)
	}
	_ = tw.Flush()
}

func printUsage(w io.Writer) {
	lines := []string{
		"tfs - CLI for TFS/Azure DevOps Server",
		"",
		"Usage:",
		"  tfs wiql \"<WIQL>\" [--project P] [--top N] [--json]              Run a WIQL query and list matching items.",
		"  tfs view <id> [--fields f1,f2,...] [--expand relations|all|none] [--json]  Show a work item by ID.",
		"  tfs update <id> --set \"Field=Value\" ... [--add-comment \"text\"] [--parent <id>] [--parent-rel <rel>] [--json] [--yes]  Update fields/comments/parent.",
		"  tfs create --type \"<WorkItemType>\" --title \"<Title>\" [--set \"Field=Value\"...] [--assigned-to \"Owner\"] [--parent <id>] [--json]  Create a work item.",
		"  tfs delete <id> --yes [--destroy] [--json]                         Delete a work item; --destroy attempts permanent removal.",
		"  tfs pr create --repository \"<Repo>\" --source \"<Branch>\" --target \"<Branch>\" --title \"<Title>\" [--description \"<Text>\"] [--draft] [--work-item <ID> ...] [--auto-complete] [--json]  Create a pull request.",
		"  tfs pr show <URL | ID> [--repository \"<Repo>\"] [--max-threads N] [--git-diff] [--json]  Show pull request details: repo, branches, title, work items, comments, optional git diff.",
		"  tfs pr comment <URL | ID> --content \"<text>\" [--repository \"<Repo>\"] [--status active|resolved|closed] [--json]  Post a comment thread on a pull request. Use --content - for stdin or --content-file <path> for file input.",
		"  tfs search --query \"<text>\" [--project P] [--top N] [--json]     Search by Title/Description.",
		"  tfs my [--top N] [--type \"<Type>\"] [--exclude-state \"<State>\"] [--all-states] [--json]  List my items in the current project (default states: Разработка, Выполняется).",
		"  tfs show <id> [--children-rel <rel>] [--max-children N] [--json]  Show details and child items.",
		"  tfs types [--project P] [--json]                                   List work item types for the project.",
		"  tfs whoami [--json]                                                Show the identity resolved from PAT.",
		"  tfs config view [--json]                                           Show config (PAT redacted).",
		"  tfs config set --base-url <url> [--project <name>] [--pat <token>] [--json]  Save config values.",
		"",
		"WorkItemType expects the type name (value[].name). Run `tfs types` to list names for your project.",
		"",
		"Global flags:",
		"  --base-url    Base URL (overrides config/env)",
		"  --project     Project (overrides config/env)",
		"  --pat         PAT token (overrides config/env)",
		"  --json        Output JSON (set --json=false for text)",
		"  --verbose     Verbose HTTP logging (no tokens)",
		"  --insecure    Skip TLS verification",
	}
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
}
