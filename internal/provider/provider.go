package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/rawtreedb/terraform-provider-rawtree/internal/client"
	"github.com/rawtreedb/terraform-provider-rawtree/internal/resources/integrations/cloudfront_ingestion"
	"github.com/rawtreedb/terraform-provider-rawtree/internal/resources/integrations/s3_ingestion"
	"github.com/rawtreedb/terraform-provider-rawtree/internal/resources/integrations/supabase_cdc_ingestion"
	"github.com/rawtreedb/terraform-provider-rawtree/internal/resources/integrations/waf_ingestion"
	"github.com/rawtreedb/terraform-provider-rawtree/internal/resources/platform/database"
)

const (
	defaultAPIURL = "https://api.us-east-1.aws.rawtree.com"
	cliConfigPath = ".config/rtree/config.json"
)

var _ provider.Provider = &RawtreeProvider{}

type RawtreeProvider struct {
	version string
}

type RawtreeProviderModel struct {
	ManagementToken types.String `tfsdk:"management_token"`
	APIKey          types.String `tfsdk:"api_key"`
	APIURL          types.String `tfsdk:"api_url"`
	Organization    types.String `tfsdk:"organization"`
	Project         types.String `tfsdk:"project"`
}

// cliConfig represents the structure of ~/.config/rtree/config.json.
type cliConfig struct {
	Token               string `json:"token"`
	URL                 string `json:"url"`
	DefaultProject      string `json:"default_project"`
	DefaultOrganization string `json:"default_organization"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &RawtreeProvider{
			version: version,
		}
	}
}

func (p *RawtreeProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "rawtree"
	resp.Version = p.version
}

func (p *RawtreeProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage RawTree resources and external ingestion integrations.",
		Attributes: map[string]schema.Attribute{
			"management_token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Bearer credential for native RawTree resources. Can also be set with RAWTREE_MANAGEMENT_TOKEN. Integrations use their own API key.",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "API key for ingestion integrations and a fallback for native resources. Set it with RAWTREE_API_KEY or the RawTree CLI configuration.",
			},
			"api_url": schema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("RawTree API URL. Defaults to https://api.rawtree.com for native resources and %s for integrations. Can also be set via RAWTREE_URL.", defaultAPIURL),
			},
			"organization": schema.StringAttribute{
				Optional:    true,
				Description: "Organization name for integrations. Can also be set with RAWTREE_ORG.",
			},
			"project": schema.StringAttribute{
				Optional:    true,
				Description: "Project name for integrations. Can also be set with RAWTREE_PROJECT.",
			},
		},
	}
}

func (p *RawtreeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config RawtreeProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cli := loadCLIConfig()

	apiKey := resolveString(config.APIKey, "RAWTREE_API_KEY", cli.Token)

	apiURL := resolveString(config.APIURL, "RAWTREE_URL", cli.URL)
	managementToken := resolveString(config.ManagementToken, "RAWTREE_MANAGEMENT_TOKEN", apiKey)
	org := resolveString(config.Organization, "RAWTREE_ORG", cli.DefaultOrganization)
	project := resolveString(config.Project, "RAWTREE_PROJECT", cli.DefaultProject)
	platformURL, integrationURL := apiURL, apiURL
	if apiURL == "" {
		platformURL = "https://api.rawtree.com"
		integrationURL = defaultAPIURL
	}
	if managementToken == "" {
		resp.Diagnostics.AddError("Missing RawTree credential", "Set management_token or RAWTREE_MANAGEMENT_TOKEN for native resources, or configure an API key.")
		return
	}
	c := &client.Clients{
		Platform:     client.NewPlatformClient(platformURL, managementToken),
		Integrations: client.New(apiKey, integrationURL, org, project),
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *RawtreeProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		database.NewResource,
		cloudfront_ingestion.NewResource,
		s3_ingestion.NewResource,
		supabase_cdc_ingestion.NewResource,
		waf_ingestion.NewResource,
	}
}

func (p *RawtreeProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// resolveString returns the first non-empty value from: TF config, env var, fallback.
func resolveString(tfValue types.String, envVar string, fallback string) string {
	if !tfValue.IsNull() && !tfValue.IsUnknown() {
		return tfValue.ValueString()
	}
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	return fallback
}

// loadCLIConfig reads the rtree CLI configuration file.
func loadCLIConfig() cliConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return cliConfig{}
	}

	data, err := os.ReadFile(filepath.Join(home, cliConfigPath))
	if err != nil {
		return cliConfig{}
	}

	var cfg cliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cliConfig{}
	}

	return cfg
}
