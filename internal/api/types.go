package api

type WorkItem struct {
	ID        int                    `json:"id"`
	Fields    map[string]interface{} `json:"fields"`
	Relations []interface{}          `json:"relations"`
	URL       string                 `json:"url"`
}

type WorkItemType struct {
	Name           string             `json:"name"`
	ReferenceName  string             `json:"referenceName"`
	Description    string             `json:"description"`
	Color          string             `json:"color"`
	IsDisabled     bool               `json:"isDisabled"`
	URL            string             `json:"url"`
	Fields         []WorkItemTypeField `json:"fields"`
	FieldInstances []WorkItemTypeField `json:"fieldInstances"`
}

type WorkItemTypesResponse struct {
	Count int            `json:"count"`
	Value []WorkItemType `json:"value"`
}

type WorkItemTypeField struct {
	Name          string `json:"name"`
	ReferenceName string `json:"referenceName"`
}

type WorkItemReference struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

type WorkItemLink struct {
	Rel    string            `json:"rel"`
	Source WorkItemReference `json:"source"`
	Target WorkItemReference `json:"target"`
}

type WiqlRequest struct {
	Query string `json:"query"`
}

type WiqlResponse struct {
	QueryType       string              `json:"queryType"`
	QueryResultType string              `json:"queryResultType"`
	WorkItems       []WorkItemReference `json:"workItems"`
	WorkItemLinks   []WorkItemLink      `json:"workItemRelations"`
}

type WorkItemsBatchRequest struct {
	IDs    []int    `json:"ids"`
	Fields []string `json:"fields,omitempty"`
	Expand string   `json:"$expand,omitempty"`
}

type WorkItemsBatchResponse struct {
	Count int        `json:"count"`
	Value []WorkItem `json:"value"`
}

type Identity struct {
	ID                  string                 `json:"id"`
	Descriptor          string                 `json:"descriptor"`
	SubjectDescriptor   string                 `json:"subjectDescriptor"`
	ProviderDisplayName string                 `json:"providerDisplayName"`
	Properties          map[string]interface{} `json:"properties"`
}

type IdentitiesResponse struct {
	Count int        `json:"count"`
	Value []Identity `json:"value"`
}

type Profile struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
	ID           string `json:"id"`
}

type HeaderIdentity struct {
	ID         string
	UniqueName string
	Raw        string
}
