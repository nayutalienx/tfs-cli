package api

type WorkItem struct {
	ID        int                    `json:"id"`
	Fields    map[string]interface{} `json:"fields"`
	Relations []interface{}          `json:"relations"`
	URL       string                 `json:"url"`
}

type WorkItemType struct {
	Name           string              `json:"name"`
	ReferenceName  string              `json:"referenceName"`
	Description    string              `json:"description"`
	Color          string              `json:"color"`
	IsDisabled     bool                `json:"isDisabled"`
	URL            string              `json:"url"`
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

type Link struct {
	Href string `json:"href"`
}

type ResourceRef struct {
	ID  string `json:"id,omitempty"`
	URL string `json:"url,omitempty"`
}

type IdentityRef struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	UniqueName  string `json:"uniqueName,omitempty"`
}

type GitRepository struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	RemoteURL string `json:"remoteUrl"`
	WebURL    string `json:"webUrl"`
}

type GitPullRequest struct {
	PullRequestID     int                              `json:"pullRequestId"`
	CodeReviewID      int                              `json:"codeReviewId"`
	Status            string                           `json:"status"`
	Title             string                           `json:"title"`
	Description       string                           `json:"description"`
	SourceRefName     string                           `json:"sourceRefName"`
	TargetRefName     string                           `json:"targetRefName"`
	IsDraft           bool                             `json:"isDraft"`
	URL               string                           `json:"url"`
	Repository        GitRepository                    `json:"repository"`
	Links             map[string]Link                  `json:"_links"`
	CreatedBy         map[string]interface{}           `json:"createdBy"`
	AutoCompleteSetBy *IdentityRef                     `json:"autoCompleteSetBy,omitempty"`
	CompletionOptions *GitPullRequestCompletionOptions `json:"completionOptions,omitempty"`
	WorkItemRefs      []ResourceRef                    `json:"workItemRefs,omitempty"`
}

type CreatePullRequestRequest struct {
	SourceRefName string        `json:"sourceRefName"`
	TargetRefName string        `json:"targetRefName"`
	Title         string        `json:"title"`
	Description   string        `json:"description,omitempty"`
	IsDraft       bool          `json:"isDraft,omitempty"`
	WorkItemRefs  []ResourceRef `json:"workItemRefs,omitempty"`
}

type GitPullRequestCompletionOptions struct {
	DeleteSourceBranch  bool `json:"deleteSourceBranch,omitempty"`
	SquashMerge         bool `json:"squashMerge,omitempty"`
	TransitionWorkItems bool `json:"transitionWorkItems,omitempty"`
}

type UpdatePullRequestRequest struct {
	AutoCompleteSetBy *IdentityRef                     `json:"autoCompleteSetBy,omitempty"`
	CompletionOptions *GitPullRequestCompletionOptions `json:"completionOptions,omitempty"`
}
