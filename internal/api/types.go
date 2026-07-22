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

type WorkItemComment struct {
	Revision    int                    `json:"revision"`
	Text        string                 `json:"text"`
	RevisedBy   map[string]interface{} `json:"revisedBy"`
	RevisedDate string                 `json:"revisedDate,omitempty"`
	URL         string                 `json:"url,omitempty"`
}

type WorkItemCommentsResponse struct {
	TotalCount        int               `json:"totalCount"`
	FromRevisionCount int               `json:"fromRevisionCount"`
	Count             int               `json:"count"`
	Value             []WorkItemComment `json:"value"`
	Comments          []WorkItemComment `json:"comments"`
}

type WikiPage struct {
	ID              int        `json:"id,omitempty"`
	Path            string     `json:"path"`
	Order           int        `json:"order,omitempty"`
	GitItemPath     string     `json:"gitItemPath,omitempty"`
	IsParentPage    bool       `json:"isParentPage,omitempty"`
	IsNonConformant bool       `json:"isNonConformant,omitempty"`
	Content         string     `json:"content"`
	URL             string     `json:"url,omitempty"`
	RemoteURL       string     `json:"remoteUrl,omitempty"`
	SubPages        []WikiPage `json:"subPages,omitempty"`
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
	PullRequestID         int                              `json:"pullRequestId"`
	CodeReviewID          int                              `json:"codeReviewId"`
	Status                string                           `json:"status"`
	Title                 string                           `json:"title"`
	Description           string                           `json:"description"`
	SourceRefName         string                           `json:"sourceRefName"`
	TargetRefName         string                           `json:"targetRefName"`
	IsDraft               bool                             `json:"isDraft"`
	URL                   string                           `json:"url"`
	CreationDate          string                           `json:"creationDate,omitempty"`
	Repository            GitRepository                    `json:"repository"`
	Links                 map[string]Link                  `json:"_links"`
	CreatedBy             map[string]interface{}           `json:"createdBy"`
	AutoCompleteSetBy     *IdentityRef                     `json:"autoCompleteSetBy,omitempty"`
	CompletionOptions     *GitPullRequestCompletionOptions `json:"completionOptions,omitempty"`
	WorkItemRefs          []ResourceRef                    `json:"workItemRefs,omitempty"`
	LastMergeSourceCommit *GitCommitRef                    `json:"lastMergeSourceCommit,omitempty"`
	LastMergeTargetCommit *GitCommitRef                    `json:"lastMergeTargetCommit,omitempty"`
}

type GitCommitRef struct {
	CommitID string `json:"commitId"`
	URL      string `json:"url"`
}

type GitChangeItem struct {
	ObjectID         string `json:"objectId,omitempty"`
	OriginalObjectID string `json:"originalObjectId,omitempty"`
	GitObjectType    string `json:"gitObjectType,omitempty"`
	CommitID         string `json:"commitId,omitempty"`
	Path             string `json:"path,omitempty"`
	URL              string `json:"url,omitempty"`
}

type GitChange struct {
	ChangeID         int           `json:"changeId"`
	ChangeType       string        `json:"changeType"`
	Item             GitChangeItem `json:"item"`
	ChangeTrackingID int           `json:"changeTrackingId,omitempty"`
	Patch            string        `json:"patch,omitempty"`
}

type GitPullRequestIteration struct {
	ID              int           `json:"id"`
	CreatedDate     string        `json:"createdDate,omitempty"`
	SourceRefCommit *GitCommitRef `json:"sourceRefCommit,omitempty"`
	TargetRefCommit *GitCommitRef `json:"targetRefCommit,omitempty"`
}

type GitPullRequestIterationsResponse struct {
	Count int                       `json:"count"`
	Value []GitPullRequestIteration `json:"value"`
}

type GitPullRequestIterationChanges struct {
	ChangeEntries []GitChange `json:"changeEntries"`
}

type GitItem struct {
	ObjectID      string `json:"objectId,omitempty"`
	GitObjectType string `json:"gitObjectType,omitempty"`
	CommitID      string `json:"commitId,omitempty"`
	Path          string `json:"path,omitempty"`
	Content       string `json:"content,omitempty"`
	URL           string `json:"url,omitempty"`
}

type GitPullRequestComment struct {
	ID              int                    `json:"id"`
	Content         string                 `json:"content"`
	CommentType     string                 `json:"commentType"`
	Author          map[string]interface{} `json:"author"`
	PublishedDate   string                 `json:"publishedDate,omitempty"`
	LastUpdatedDate string                 `json:"lastUpdatedDate,omitempty"`
	IsDeleted       bool                   `json:"isDeleted,omitempty"`
	ParentCommentID int                    `json:"parentCommentId,omitempty"`
}

type GitPullRequestThread struct {
	ID              int                     `json:"id"`
	Status          string                  `json:"status,omitempty"`
	IsDeleted       bool                    `json:"isDeleted,omitempty"`
	Comments        []GitPullRequestComment `json:"comments"`
	PublishedDate   string                  `json:"publishedDate,omitempty"`
	LastUpdatedDate string                  `json:"lastUpdatedDate,omitempty"`
	Context         map[string]interface{}  `json:"context,omitempty"`
}

type GitPullRequestThreadsResponse struct {
	Count int                    `json:"count"`
	Value []GitPullRequestThread `json:"value"`
}

type GitPullRequestWorkItemsResponse struct {
	Count int           `json:"count"`
	Value []ResourceRef `json:"value"`
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

type CreatePullRequestThreadRequest struct {
	Comments []CreatePullRequestComment `json:"comments"`
	Status   int                        `json:"status,omitempty"`
}

type CreatePullRequestComment struct {
	ParentCommentID int    `json:"parentCommentId,omitempty"`
	Content         string `json:"content"`
	CommentType     int    `json:"commentType,omitempty"`
}
