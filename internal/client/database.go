package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Clients keeps the platform credential separate from credentials deployed to AWS.
type Clients struct {
	Platform     *PlatformClient
	Integrations *RawtreeClient
}

type PlatformClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewPlatformClient(baseURL, token string) *PlatformClient {
	return &PlatformClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Database identifies a database within its organization and cluster.
type Database struct {
	Organization string
	Cluster      string
	Name         string
}

func (d Database) ID() string {
	return strings.Join([]string{d.Organization, d.Cluster, d.Name}, "/")
}

func (c *PlatformClient) DatabaseExists(ctx context.Context, db Database) (bool, error) {
	var result struct {
		Databases []struct{ Name string } `json:"databases"`
	}
	if err := c.databaseRequest(ctx, db, http.MethodGet, nil, &result); err != nil {
		return false, err
	}
	if result.Databases == nil {
		return false, fmt.Errorf("RawTree returned an incomplete database list")
	}
	for _, item := range result.Databases {
		if item.Name == "" {
			return false, fmt.Errorf("RawTree returned a database without a name")
		}
		if item.Name == db.Name {
			return true, nil
		}
	}
	return false, nil
}

func (c *PlatformClient) CreateDatabase(ctx context.Context, db Database) error {
	body, err := json.Marshal(struct {
		Name string `json:"name"`
	}{db.Name})
	if err != nil {
		return err
	}
	var result struct{ Database struct{ Name string } }
	if err := c.databaseRequest(ctx, db, http.MethodPost, body, &result); err != nil {
		return err
	}
	if result.Database.Name != db.Name {
		return fmt.Errorf("RawTree did not confirm creation of database %q", db.Name)
	}
	return nil
}

func (c *PlatformClient) DeleteDatabase(ctx context.Context, db Database) error {
	var result struct{ Deleted *bool }
	if err := c.databaseRequest(ctx, db, http.MethodDelete, nil, &result); err != nil {
		return err
	}
	// The API returns deleted=false if the database is already absent.
	if result.Deleted == nil {
		return fmt.Errorf("RawTree did not confirm deletion of database %q", db.Name)
	}
	return nil
}

func (c *PlatformClient) databaseRequest(ctx context.Context, db Database, method string, body []byte, result any) error {
	endpoint := c.baseURL + "/v1/databases"
	if method == http.MethodDelete {
		endpoint += "/" + url.PathEscape(db.Name)
	}
	scope := url.Values{"organization": {db.Organization}, "cluster": {db.Cluster}}
	req, err := http.NewRequestWithContext(ctx, method, endpoint+"?"+scope.Encode(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "terraform-provider-rawtree")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var problem struct{ Message string }
		_ = json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&problem)
		return fmt.Errorf("RawTree %s %s returned HTTP %d: %s", method, req.URL.Path, resp.StatusCode, problem.Message)
	}
	return json.NewDecoder(resp.Body).Decode(result)
}
