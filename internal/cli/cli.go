package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"

	"tfs-cli/internal/api"
	"tfs-cli/internal/config"
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
	cfg       config.Config
	jsonMode  bool
	verbose   bool
	insecure  bool
	stdout    io.Writer
	stderr    io.Writer
	project   string
	baseURL   string
	pat       string
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
	case "search":
		return runSearch(args[1:], stdout, stderr)
	case "my":
		return runMy(args[1:], stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
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
	if len(sets.values) == 0 && *comment == "" {
		output.WriteError(stderr, errs.New("invalid_args", "at least one --set or --add-comment is required", nil), flags.json)
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

	patch, err := buildPatch(sets.values, *comment)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	client, err := api.NewClient(ctx.baseURL, ctx.project, ctx.pat, ctx.insecure, ctx.verbose, stderr)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	wi, err := client.UpdateWorkItem(context.Background(), id, patch)
	if err != nil {
		output.WriteError(stderr, err, ctx.jsonMode)
		return 1
	}
	return renderWorkItem(ctx, wi)
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
	flags["yes"] = false
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
		"  tfs update <id> --set \"Field=Value\" ... [--add-comment \"text\"] [--json] [--yes]  Update fields/comments.",
		"  tfs create --type \"<WorkItemType>\" --title \"<Title>\" [--set \"Field=Value\"...] [--assigned-to \"Owner\"] [--parent <id>] [--json]  Create a work item.",
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
