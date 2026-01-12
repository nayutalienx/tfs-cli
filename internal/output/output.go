package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"tfs-cli/internal/api"
	"tfs-cli/internal/errs"
)

type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

func WriteError(w io.Writer, err error, jsonMode bool) {
	if jsonMode {
		env := ErrorEnvelope{Error: ErrorDetail{Code: "internal_error", Message: err.Error()}}
		if appErr, ok := err.(errs.AppError); ok {
			env.Error.Code = appErr.Code
			env.Error.Message = appErr.Message
			env.Error.Details = appErr.Details
		}
		data, _ := json.Marshal(env)
		fmt.Fprintln(w, string(data))
		return
	}
	fmt.Fprintln(w, err.Error())
}

func PrintJSON(w io.Writer, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

type WorkItem struct {
	ID            int                    `json:"id"`
	Type          *string                `json:"type"`
	State         *string                `json:"state"`
	Title         *string                `json:"title"`
	AssignedTo    *string                `json:"assignedTo"`
	AreaPath      *string                `json:"areaPath"`
	IterationPath *string                `json:"iterationPath"`
	Tags          *string                `json:"tags"`
	URL           *string                `json:"url"`
	Fields        map[string]interface{} `json:"fields"`
}

func NormalizeWorkItem(raw api.WorkItem) WorkItem {
	fields := raw.Fields
	if fields == nil {
		fields = map[string]interface{}{}
	}
	wiType := fieldString(fields, "System.WorkItemType")
	state := fieldString(fields, "System.State")
	title := fieldString(fields, "System.Title")
	area := fieldString(fields, "System.AreaPath")
	iteration := fieldString(fields, "System.IterationPath")
	tags := fieldString(fields, "System.Tags")
	assigned := identityString(fields["System.AssignedTo"])
	url := raw.URL

	return WorkItem{
		ID:            raw.ID,
		Type:          wiType,
		State:         state,
		Title:         title,
		AssignedTo:    assigned,
		AreaPath:      area,
		IterationPath: iteration,
		Tags:          tags,
		URL:           stringPtr(url),
		Fields:        fields,
	}
}

func PrintTable(w io.Writer, items []WorkItem) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTYPE\tSTATE\tTITLE\tASSIGNED")
	for _, item := range items {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			item.ID,
			stringValue(item.Type),
			stringValue(item.State),
			stringValue(item.Title),
			stringValue(item.AssignedTo),
		)
	}
	_ = tw.Flush()
}

func identityString(v interface{}) *string {
	switch val := v.(type) {
	case string:
		return stringPtr(val)
	case map[string]interface{}:
		if dn, ok := val["displayName"].(string); ok && dn != "" {
			if un, ok := val["uniqueName"].(string); ok && un != "" {
				combined := fmt.Sprintf("%s<%s>", dn, un)
				return stringPtr(combined)
			}
			return stringPtr(dn)
		}
		if un, ok := val["uniqueName"].(string); ok && un != "" {
			return stringPtr(un)
		}
	}
	return nil
}

func fieldString(fields map[string]interface{}, key string) *string {
	if v, ok := fields[key]; ok {
		if s, ok := v.(string); ok {
			return stringPtr(s)
		}
	}
	return nil
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
