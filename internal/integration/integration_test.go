//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"tfs-cli/internal/api"
	"tfs-cli/internal/cli"
)

func TestTFSIntegration(t *testing.T) {
	baseURL := requireEnv(t, "TFS_BASE_URL")
	project := requireEnv(t, "TFS_PROJECT")
	pat := requireEnv(t, "TFS_PAT")
	wiType := requireEnv(t, "TFS_WIT_TYPE")
	taskType := strings.TrimSpace(os.Getenv("TFS_TASK_TYPE"))
	if taskType == "" {
		taskType = "Задача"
	}
	activityName := strings.TrimSpace(os.Getenv("TFS_TASK_ACTIVITY_NAME"))
	if activityName == "" {
		activityName = "Активность"
	}
	activityValue := strings.TrimSpace(os.Getenv("TFS_TASK_ACTIVITY_VALUE"))
	if activityValue == "" {
		activityValue = "Разработка"
	}
	remainingWorkName := strings.TrimSpace(os.Getenv("TFS_TASK_REMAINING_WORK_NAME"))
	if remainingWorkName == "" {
		remainingWorkName = "Оставшаяся работа"
	}
	assignedTo := strings.TrimSpace(os.Getenv("TFS_ASSIGNED_TO"))
	insecure := strings.TrimSpace(os.Getenv("TFS_INSECURE"))

	t.Setenv("TFS_BASE_URL", baseURL)
	t.Setenv("TFS_PROJECT", project)
	t.Setenv("TFS_PAT", pat)

	globalFlags := []string{}
	if insecure != "" && insecure != "0" {
		globalFlags = append(globalFlags, "--insecure")
	}
	globalFlags = append(globalFlags, "--verbose")

	token := fmt.Sprintf("tfs-cli-itest-%d", time.Now().UnixNano())
	createTitle := "TFS CLI integration " + token
	updateTitle := "TFS CLI integration updated " + token

	createArgs := append([]string{"create", "--type", wiType, "--title", createTitle, "--set", "System.Tags=tfs-cli-itest", "--set", "System.Description=Integration test description"}, globalFlags...)
	if assignedTo != "" {
		createArgs = append(createArgs, "--assigned-to", assignedTo)
	}
	createOut := runCLI(t, createArgs)
	createdID := parseWorkItemID(t, createOut)
	if createdID == 0 {
		t.Fatalf("create returned empty id")
	}

	viewArgs := append([]string{"view", fmt.Sprintf("%d", createdID), "--fields", "System.Title,System.AssignedTo"}, globalFlags...)
	viewOut := runCLI(t, viewArgs)
	if gotTitle := parseWorkItemTitle(t, viewOut); gotTitle == "" {
		t.Fatalf("view returned empty title")
	}

	updateArgs := append([]string{"update", fmt.Sprintf("%d", createdID), "--set", "System.Title=" + updateTitle, "--add-comment", "integration test"}, globalFlags...)
	updateOut := runCLI(t, updateArgs)
	if gotTitle := parseWorkItemTitle(t, updateOut); gotTitle != updateTitle {
		t.Fatalf("update title mismatch: %q", gotTitle)
	}

	taskTitle := "TFS CLI integration task " + token
	taskArgs := append([]string{
		"create",
		"--type", taskType,
		"--title", taskTitle,
		"--parent", fmt.Sprintf("%d", createdID),
		"--set", "System.Tags=tfs-cli-itest",
		"--set", "System.Description=Integration test task description",
	}, globalFlags...)
	apiClient, err := api.NewClient(baseURL, project, pat, insecure != "" && insecure != "0", true, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("api client error: %v", err)
	}
	fieldRefs, err := resolveFieldRefs(apiClient, taskType, []string{activityName, remainingWorkName})
	if err != nil {
		t.Fatalf("resolve field refs: %v", err)
	}
	taskArgs = append(taskArgs,
		"--set", fmt.Sprintf("%s=%s", fieldRefs[activityName], activityValue),
		"--set", fmt.Sprintf("%s=%d", fieldRefs[remainingWorkName], 4),
	)
	if assignedTo != "" {
		taskArgs = append(taskArgs, "--assigned-to", assignedTo)
	}
	taskOut := runCLI(t, taskArgs)
	taskID := parseWorkItemID(t, taskOut)
	if taskID == 0 {
		t.Fatalf("task create returned empty id")
	}
	taskCommentArgs := append([]string{"update", fmt.Sprintf("%d", taskID), "--add-comment", "integration test task comment"}, globalFlags...)
	runCLI(t, taskCommentArgs)

	wiqlQuery := fmt.Sprintf("SELECT [System.Id] FROM WorkItems WHERE [System.Id] = %d", createdID)
	wiqlArgs := append([]string{"wiql", wiqlQuery, "--top", "1"}, globalFlags...)
	wiqlOut := runCLI(t, wiqlArgs)
	ids := parseListIDs(t, wiqlOut)
	if !containsID(ids, createdID) {
		t.Fatalf("wiql results missing id %d", createdID)
	}

	searchArgs := append([]string{"search", "--query", token, "--top", "20"}, globalFlags...)
	searchOut := runCLI(t, searchArgs)
	searchIDs := parseListIDs(t, searchOut)
	if !containsID(searchIDs, createdID) {
		t.Fatalf("search results missing id %d", createdID)
	}
}

func requireEnv(t *testing.T, key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		t.Skipf("missing %s", key)
	}
	return value
}

func runCLI(t *testing.T, args []string) string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := cli.Run(args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("command failed: %v\nstdout: %s\nstderr: %s", args, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func parseWorkItemID(t *testing.T, payload string) int {
	t.Helper()
	var resp struct {
		WorkItem struct {
			ID int `json:"id"`
		} `json:"workItem"`
	}
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	return resp.WorkItem.ID
}

func parseWorkItemTitle(t *testing.T, payload string) string {
	t.Helper()
	var resp struct {
		WorkItem struct {
			Title *string `json:"title"`
		} `json:"workItem"`
	}
	if err := json.Unmarshal([]byte(payload), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if resp.WorkItem.Title == nil {
		return ""
	}
	return *resp.WorkItem.Title
}

func parseListIDs(t *testing.T, payload string) []int {
	t.Helper()
	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &items); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	ids := make([]int, 0, len(items))
	for _, item := range items {
		if raw, ok := item["id"]; ok {
			switch v := raw.(type) {
			case float64:
				ids = append(ids, int(v))
			case int:
				ids = append(ids, v)
			}
		}
	}
	return ids
}

func containsID(ids []int, id int) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func resolveFieldRefs(client *api.Client, workItemType string, fieldNames []string) (map[string]string, error) {
	types, err := client.ListWorkItemTypes(context.Background())
	if err != nil {
		return nil, err
	}
	var target *api.WorkItemType
	for i := range types {
		if types[i].Name == workItemType {
			target = &types[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("work item type not found: %s", workItemType)
	}
	fields := target.Fields
	if len(fields) == 0 {
		fields = target.FieldInstances
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("work item type fields not available for: %s", workItemType)
	}
	out := map[string]string{}
	for _, name := range fieldNames {
		for _, f := range fields {
			if f.Name == name {
				out[name] = f.ReferenceName
				break
			}
		}
		if out[name] == "" {
			return nil, fmt.Errorf("field not found on type %s: %s", workItemType, name)
		}
	}
	return out, nil
}
