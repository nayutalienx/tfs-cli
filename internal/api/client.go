package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tfs-cli/internal/errs"
)

const (
	defaultTimeout   = 30 * time.Second
	maxRetries       = 4
	initialBackoff   = 500 * time.Millisecond
	maxBackoff       = 5 * time.Second
	defaultAPIVersion = "6.0"
)

type Client struct {
	baseURL string
	project string
	pat     string
	client  *http.Client
	verbose bool
	log     io.Writer
}

func NewClient(baseURL, project, pat string, insecure bool, verbose bool, log io.Writer) (*Client, error) {
	if baseURL == "" {
		return nil, errs.New("config_missing", "base URL is required", nil)
	}
	if pat == "" {
		return nil, errs.New("config_missing", "PAT is required", nil)
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
		},
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		project: project,
		pat:     pat,
		client: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transport,
		},
		verbose: verbose,
		log:     log,
	}, nil
}

func (c *Client) WithProject(project string) *Client {
	clone := *c
	clone.project = project
	return &clone
}

func (c *Client) Wiql(ctx context.Context, query string, top int) (WiqlResponse, error) {
	path := fmt.Sprintf("%s/_apis/wit/wiql", c.project)
	params := url.Values{}
	params.Set("api-version", defaultAPIVersion)
	if top > 0 {
		params.Set("$top", strconv.Itoa(top))
	}
	body, err := json.Marshal(WiqlRequest{Query: query})
	if err != nil {
		return WiqlResponse{}, err
	}
	respBody, err := c.do(ctx, http.MethodPost, path, params, body, "application/json")
	if err != nil {
		return WiqlResponse{}, err
	}
	var resp WiqlResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return WiqlResponse{}, err
	}
	return resp, nil
}

func (c *Client) GetWorkItem(ctx context.Context, id int, fields []string, expand string) (WorkItem, error) {
	path := fmt.Sprintf("%s/_apis/wit/workitems/%d", c.project, id)
	params := url.Values{}
	params.Set("api-version", defaultAPIVersion)
	if len(fields) > 0 {
		params.Set("fields", strings.Join(fields, ","))
	}
	if expand != "" {
		params.Set("$expand", expand)
	}
	respBody, err := c.do(ctx, http.MethodGet, path, params, nil, "")
	if err != nil {
		return WorkItem{}, err
	}
	var wi WorkItem
	if err := json.Unmarshal(respBody, &wi); err != nil {
		return WorkItem{}, err
	}
	return wi, nil
}

func (c *Client) GetWorkItemsBatch(ctx context.Context, ids []int, fields []string) ([]WorkItem, error) {
	path := fmt.Sprintf("%s/_apis/wit/workitemsbatch", c.project)
	params := url.Values{}
	params.Set("api-version", defaultAPIVersion)
	payload := WorkItemsBatchRequest{IDs: ids, Fields: fields}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	respBody, err := c.do(ctx, http.MethodPost, path, params, body, "application/json")
	if err != nil {
		return nil, err
	}
	var items []WorkItem
	if err := json.Unmarshal(respBody, &items); err != nil {
		var wrapped WorkItemsBatchResponse
		if wrapErr := json.Unmarshal(respBody, &wrapped); wrapErr != nil {
			return nil, err
		}
		items = wrapped.Value
	}
	return items, nil
}

func (c *Client) UpdateWorkItem(ctx context.Context, id int, patch []map[string]interface{}) (WorkItem, error) {
	path := fmt.Sprintf("%s/_apis/wit/workitems/%d", c.project, id)
	params := url.Values{}
	params.Set("api-version", defaultAPIVersion)
	body, err := json.Marshal(patch)
	if err != nil {
		return WorkItem{}, err
	}
	respBody, err := c.do(ctx, http.MethodPatch, path, params, body, "application/json-patch+json")
	if err != nil {
		return WorkItem{}, err
	}
	var wi WorkItem
	if err := json.Unmarshal(respBody, &wi); err != nil {
		return WorkItem{}, err
	}
	return wi, nil
}

func (c *Client) CreateWorkItem(ctx context.Context, wiType string, patch []map[string]interface{}) (WorkItem, error) {
	path := fmt.Sprintf("%s/_apis/wit/workitems/$%s", c.project, url.PathEscape(wiType))
	params := url.Values{}
	params.Set("api-version", defaultAPIVersion)
	body, err := json.Marshal(patch)
	if err != nil {
		return WorkItem{}, err
	}
	respBody, err := c.do(ctx, http.MethodPost, path, params, body, "application/json-patch+json")
	if err != nil {
		return WorkItem{}, err
	}
	var wi WorkItem
	if err := json.Unmarshal(respBody, &wi); err != nil {
		return WorkItem{}, err
	}
	return wi, nil
}

func (c *Client) ProfileMe(ctx context.Context) (Profile, error) {
	base := c.profileBaseURL()
	path := joinURL(base, "_apis/profile/profiles/me")
	params := url.Values{}
	params.Set("api-version", defaultAPIVersion)
	respBody, err := c.doFullURL(ctx, http.MethodGet, path, params, nil, "")
	if err != nil {
		return Profile{}, err
	}
	var profile Profile
	if err := json.Unmarshal(respBody, &profile); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (c *Client) ListWorkItemTypes(ctx context.Context) ([]WorkItemType, error) {
	path := fmt.Sprintf("%s/_apis/wit/workitemtypes", c.project)
	params := url.Values{}
	params.Set("api-version", defaultAPIVersion)
	respBody, err := c.do(ctx, http.MethodGet, path, params, nil, "")
	if err != nil {
		return nil, err
	}
	var resp WorkItemTypesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}
	return resp.Value, nil
}

func (c *Client) WhoamiFromHeaders(ctx context.Context) (HeaderIdentity, error) {
	if c.project == "" {
		return HeaderIdentity{}, errs.New("config_missing", "project is required", nil)
	}
	path := fmt.Sprintf("%s/_apis/wit/workitemtypes", c.project)
	params := url.Values{}
	params.Set("api-version", defaultAPIVersion)
	headers, _, err := c.doWithHeaders(ctx, http.MethodGet, path, params, nil, "")
	if err != nil {
		return HeaderIdentity{}, err
	}
	raw := headers.Get("X-Vss-Userdata")
	if raw == "" {
		return HeaderIdentity{}, errs.New("whoami_unavailable", "X-Vss-Userdata header missing", nil)
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) == 2 {
		return HeaderIdentity{ID: parts[0], UniqueName: parts[1], Raw: raw}, nil
	}
	return HeaderIdentity{Raw: raw, UniqueName: raw}, nil
}

func (c *Client) ResolveIdentityByID(ctx context.Context, id string) (*Identity, error) {
	if id == "" {
		return nil, errs.New("invalid_args", "identity id is required", nil)
	}
	path := "_apis/identities"
	params := url.Values{}
	params.Set("api-version", defaultAPIVersion)
	params.Set("identityIds", id)
	respBody, err := c.do(ctx, http.MethodGet, path, params, nil, "")
	if err != nil {
		return nil, err
	}
	var resp IdentitiesResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}
	if len(resp.Value) == 0 {
		return nil, errs.New("identity_not_found", "identity not found", id)
	}
	return &resp.Value[0], nil
}

func (c *Client) WorkItemURL(id int) string {
	return joinURL(c.baseURL, fmt.Sprintf("_apis/wit/workItems/%d", id))
}

func (c *Client) profileBaseURL() string {
	lower := strings.ToLower(c.baseURL)
	if strings.Contains(lower, "dev.azure.com") || strings.Contains(lower, "visualstudio.com") {
		return "https://app.vssps.visualstudio.com"
	}
	// TODO: confirm on-prem profile base URL; using base URL as fallback.
	return c.baseURL
}

func (c *Client) do(ctx context.Context, method, path string, params url.Values, body []byte, contentType string) ([]byte, error) {
	url := joinURL(c.baseURL, path)
	return c.doFullURL(ctx, method, url, params, body, contentType)
}

func (c *Client) doWithHeaders(ctx context.Context, method, path string, params url.Values, body []byte, contentType string) (http.Header, []byte, error) {
	url := joinURL(c.baseURL, path)
	return c.doFullURLWithHeaders(ctx, method, url, params, body, contentType)
}

func (c *Client) doFullURL(ctx context.Context, method, fullURL string, params url.Values, body []byte, contentType string) ([]byte, error) {
	headers, respBody, err := c.doFullURLWithHeaders(ctx, method, fullURL, params, body, contentType)
	_ = headers
	return respBody, err
}

func (c *Client) doFullURLWithHeaders(ctx context.Context, method, fullURL string, params url.Values, body []byte, contentType string) (http.Header, []byte, error) {
	if params != nil && len(params) > 0 {
		fullURL = fullURL + "?" + params.Encode()
	}
	var lastErr error
	backoff := initialBackoff
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := c.newRequest(ctx, method, fullURL, body, contentType)
		if err != nil {
			return nil, nil, err
		}
		if c.verbose {
			c.logRequest(req, body)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, nil, err
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, nil, readErr
		}
		if c.verbose {
			c.logResponse(resp, respBody)
		}
		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			return resp.Header, respBody, nil
		}

		if shouldRetry(resp.StatusCode) && attempt < maxRetries {
			wait := retryAfter(resp.Header.Get("Retry-After"))
			if wait == 0 {
				wait = backoff
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			lastErr = errs.New("http_retry", fmt.Sprintf("retryable status %d", resp.StatusCode), string(respBody))
			time.Sleep(wait)
			continue
		}
		return nil, nil, errs.New("http_error", fmt.Sprintf("request failed with status %d", resp.StatusCode), string(respBody))
	}
	if lastErr != nil {
		return nil, nil, lastErr
	}
	return nil, nil, errs.New("http_error", "request failed", nil)
}

func (c *Client) newRequest(ctx context.Context, method, fullURL string, body []byte, contentType string) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Authorization", "Basic "+basicAuthToken(c.pat))
	return req, nil
}

func basicAuthToken(pat string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(":" + pat))
	return encoded
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func retryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func (c *Client) logRequest(req *http.Request, body []byte) {
	if c.log == nil {
		return
	}
	fmt.Fprintf(c.log, "> %s %s\n", req.Method, req.URL.String())
	for key, values := range req.Header {
		if strings.EqualFold(key, "Authorization") {
			continue
		}
		for _, value := range values {
			fmt.Fprintf(c.log, "> %s: %s\n", key, value)
		}
	}
	if len(body) > 0 {
		fmt.Fprintf(c.log, "> body: %s\n", truncateBody(body))
	}
}

func (c *Client) logResponse(resp *http.Response, body []byte) {
	if c.log == nil {
		return
	}
	fmt.Fprintf(c.log, "< %s\n", resp.Status)
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Fprintf(c.log, "< %s: %s\n", key, value)
		}
	}
	if len(body) > 0 {
		fmt.Fprintf(c.log, "< body: %s\n", truncateBody(body))
	}
}

func truncateBody(body []byte) string {
	const limit = 2048
	if len(body) <= limit {
		return string(body)
	}
	return string(body[:limit]) + "..."
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}
