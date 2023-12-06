package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &aapProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &aapProvider{
			version: version,
		}
	}
}

// aapProvider is the provider implementation.
type aapProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// Metadata returns the provider type name.
func (p *aapProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "aap"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *aapProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional: true,
			},
			"username": schema.StringAttribute{
				Optional: true,
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
			"insecure_skip_verify": schema.BoolAttribute{
				Optional: true,
			},
			"timeout": schema.Int64Attribute{
				Optional: true,
				Description: "Timeout specifies a time limit for requests made to the AAP server." +
					" A Timeout of zero means no timeout.",
			},
		},
	}
}

func AddConfigurationAttributeError(resp *provider.ConfigureResponse, name, envName string, isUnknown bool) {
	if isUnknown {
		resp.Diagnostics.AddAttributeError(
			path.Root(name),
			"Unknown AAP API "+name,
			fmt.Sprintf("The provider cannot create the AAP API client as there is an unknown configuration value for the AAP API %s. "+
				"Either target apply the source of the value first, set the value statically in the configuration,"+
				" or use the %s environment variable.", name, envName),
		)
	} else {
		resp.Diagnostics.AddAttributeError(
			path.Root(name),
			"Missing AAP API "+name,
			fmt.Sprintf("The provider cannot create the AAP API client as there is a missing or empty value for the AAP API %s. "+
				"Set the value in the configuration or use the %s environment variable. "+
				"If either is already set, ensure the value is not empty.", name, envName),
		)
	}
}

func (p *aapProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration
	var config aapProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.
	config.checkUnknownValue(resp)
	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	host := os.Getenv("AAP_HOST")
	username := os.Getenv("AAP_USERNAME")
	password := os.Getenv("AAP_PASSWORD")
	var insecureSkipVerify = false
	var err error
	rawInsecureSkipVerify := os.Getenv("AAP_INSECURE_SKIP_VERIFY")
	if rawInsecureSkipVerify != "" {
		insecureSkipVerify, err = strconv.ParseBool(rawInsecureSkipVerify)
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("insecure_skip_verify"),
				"Invalid value for insecure_skip_verify",
				"The provider cannot create the AAP API client as the value provided for insecure_skip_verify is not a valid boolean.",
			)
			return
		}
	}

	rawTimeout := os.Getenv("AAP_TIMEOUT")
	var timeout int64
	if rawTimeout != "" {
		// convert string into int64 value
		timeout, err = strconv.ParseInt(rawTimeout, 10, 64)
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("timeout"),
				"Invalid value for timeout",
				"The provider cannot create the AAP API client as the value provided for timeout is not a valid int64 value.",
			)
			return
		}
	}
	config.ReadValues(&host, &username, &password, &insecureSkipVerify, &timeout)

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if len(host) == 0 {
		AddConfigurationAttributeError(resp, "host", "AAP_HOST", false)
	}

	if len(username) == 0 {
		AddConfigurationAttributeError(resp, "username", "AAP_USERNAME", false)
	}

	if len(password) == 0 {
		AddConfigurationAttributeError(resp, "password", "AAP_PASSWORD", false)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create a new http client using the configuration values
	client, err := NewClient(host, &username, &password, insecureSkipVerify, timeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create AAP API Client",
			"An unexpected error occurred when creating the AAP API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"http Client Error: "+err.Error(),
		)
		return
	}

	// Make the http client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *aapProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewInventoryDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *aapProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewJobResource,
	}
}

// aapProviderModel maps provider schema data to a Go type.
type aapProviderModel struct {
	Host               types.String `tfsdk:"host"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
	Timeout            types.Int64  `tfsdk:"timeout"`
}

func (p *aapProviderModel) checkUnknownValue(resp *provider.ConfigureResponse) {
	if p.Host.IsUnknown() {
		AddConfigurationAttributeError(resp, "host", "AAP_HOST", true)
	}

	if p.Username.IsUnknown() {
		AddConfigurationAttributeError(resp, "username", "AAP_USERNAME", true)
	}

	if p.Password.IsUnknown() {
		AddConfigurationAttributeError(resp, "password", "AAP_PASSWORD", true)
	}

	if p.InsecureSkipVerify.IsUnknown() {
		AddConfigurationAttributeError(resp, "insecure_skip_verify", "AAP_INSECURE_SKIP_VERIFY", true)
	}

	if p.Timeout.IsUnknown() {
		AddConfigurationAttributeError(resp, "timeout", "AAP_TIMEOUT", true)
	}
}

func (p *aapProviderModel) ReadValues(host, username, password *string, insecureSkipVerify *bool, timeout *int64) {
	if !p.Host.IsNull() {
		*host = p.Host.ValueString()
	}

	if !p.Username.IsNull() {
		*username = p.Username.ValueString()
	}

	if !p.Password.IsNull() {
		*password = p.Password.ValueString()
	}

	if !p.InsecureSkipVerify.IsNull() {
		*insecureSkipVerify = p.InsecureSkipVerify.ValueBool()
	}

	if !p.Timeout.IsNull() && !p.Timeout.IsUnknown() {
		*timeout = p.Timeout.ValueInt64()
	}
}
