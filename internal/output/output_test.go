package output

import (
	"reflect"
	"testing"

	"tfs-cli/internal/api"
)

func TestNormalizeWorkItemIdentity(t *testing.T) {
	raw := api.WorkItem{
		ID: 42,
		Fields: map[string]interface{}{
			"System.WorkItemType": "User Story",
			"System.State":        "Active",
			"System.Title":        "Test",
			"System.AssignedTo": map[string]interface{}{
				"displayName": "Alex Doe",
				"uniqueName":  "alex@example.com",
			},
		},
		URL: "http://example.com/wi/42",
	}

	norm := NormalizeWorkItem(raw)
	if norm.ID != 42 {
		t.Fatalf("expected id 42, got %d", norm.ID)
	}
	if norm.Type == nil || *norm.Type != "User Story" {
		t.Fatalf("expected type User Story, got %v", norm.Type)
	}
	if norm.AssignedTo == nil || *norm.AssignedTo != "Alex Doe<alex@example.com>" {
		t.Fatalf("unexpected assignedTo: %v", norm.AssignedTo)
	}
	if !reflect.DeepEqual(norm.Fields, raw.Fields) {
		t.Fatalf("fields mismatch")
	}
}

func TestNormalizeWorkItemAssignedString(t *testing.T) {
	raw := api.WorkItem{
		ID: 1,
		Fields: map[string]interface{}{
			"System.AssignedTo": "Pat Owner",
		},
	}
	norm := NormalizeWorkItem(raw)
	if norm.AssignedTo == nil || *norm.AssignedTo != "Pat Owner" {
		t.Fatalf("unexpected assignedTo: %v", norm.AssignedTo)
	}
}

