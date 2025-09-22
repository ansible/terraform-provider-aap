package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider            = &aapProvider{}
	_ provider.ProviderWithActions = &aapProvider{}
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
				Optional:            true,
				MarkdownDescription: "AAP Server URL. Can also be configured using the `AAP_HOSTNAME` environment variable.",
			},
			"username": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Username to use for basic authentication. " +
					"Ignored if token is set. Can also be configured by setting the `AAP_USERNAME` environment variable.",
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Password to use for basic authentication. " +
					"Ignored if token is set. Can also be configured by setting the `AAP_PASSWORD` environment variable.",
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Token to use for token authentication. " +
					"Can also be configured by setting the `AAP_TOKEN` environment variable.",
			},
			"insecure_skip_verify": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "If true, configures the provider to skip TLS certificate verification. " +
					"Can also be configured by setting the `AAP_INSECURE_SKIP_VERIFY` environment variable.",
			},
			"timeout": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: "Timeout specifies a time limit for requests made to the AAP server. " +
					"Defaults to 5 if not provided. A Timeout of zero means no timeout. " +
					"Can also be configured by setting the `AAP_TIMEOUT` environment variable",
			},
		},
	}
}

func AddConfigurationAttributeError(diags *diag.Diagnostics, name, envName string, isUnknown bool) {
	if isUnknown {
		diags.AddAttributeError(
			path.Root(name),
			"Unknown AAP API "+name,
			fmt.Sprintf("The provider cannot create the AAP API client as there is an unknown configuration value for the AAP API %s. "+
				"Either target apply the source of the value first, set the value statically in the configuration,"+
				" or use the %s environment variable.", name, envName),
		)
	} else {
		diags.AddAttributeError(
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
	config.checkUnknownValue(&resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	var host, username, password, token string
	var insecureSkipVerify bool
	var timeout int64
	config.ReadValues(&host, &username, &password, &token, &insecureSkipVerify, &timeout, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if len(host) == 0 {
		AddConfigurationAttributeError(&resp.Diagnostics, "host", "AAP_HOSTNAME", false)
	}

	if len(token) == 0 && len(username) == 0 && len(password) == 0 {
		// No authentication method at all, fail with all errors
		AddConfigurationAttributeError(&resp.Diagnostics, "token", "AAP_TOKEN", false)
		AddConfigurationAttributeError(&resp.Diagnostics, "username", "AAP_USERNAME", false)
		AddConfigurationAttributeError(&resp.Diagnostics, "password", "AAP_PASSWORD", false)
	} else if len(token) == 0 {
		// No token, but may have username and password, report error if either is missing
		if len(username) == 0 {
			AddConfigurationAttributeError(&resp.Diagnostics, "username", "AAP_USERNAME", false)
		}
		if len(password) == 0 {
			AddConfigurationAttributeError(&resp.Diagnostics, "password", "AAP_PASSWORD", false)
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create a new http client using the configuration values
	var authenticator AAPClientAuthenticator
	if len(token) > 0 {
		authenticator, diags = NewTokenAuthenticator(&token)
	} else {
		authenticator, diags = NewBasicAuthenticator(&username, &password)
	}
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	client, diags := NewClient(host, authenticator, insecureSkipVerify, timeout)
	resp.Diagnostics.Append(diags...)

	// Make the http client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client
	resp.ActionData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *aapProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewInventoryDataSource,
		NewJobTemplateDataSource,
		NewWorkflowJobTemplateDataSource,
		NewOrganizationDataSource,
		NewEDAEventStreamDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *aapProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewInventoryResource,
		NewJobResource,
		NewWorkflowJobResource,
		NewGroupResource,
		NewHostResource,
	}
}

// Actions defines the actions implemented in the provider.
func (p *aapProvider) Actions(_ context.Context) []func() action.Action {
	return []func() action.Action{
		NewEDAEventStreamPostAction,
		NewJobAction,
	}
}

// aapProviderModel maps provider schema data to a Go type.
type aapProviderModel struct {
	Host               types.String `tfsdk:"host"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	Token              types.String `tfsdk:"token"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
	Timeout            types.Int64  `tfsdk:"timeout"`
}

func (p *aapProviderModel) checkUnknownValue(diags *diag.Diagnostics) {
	if p.Host.IsUnknown() {
		AddConfigurationAttributeError(diags, "host", "AAP_HOSTNAME", true)
	}

	if p.Username.IsUnknown() {
		AddConfigurationAttributeError(diags, "username", "AAP_USERNAME", true)
	}

	if p.Password.IsUnknown() {
		AddConfigurationAttributeError(diags, "password", "AAP_PASSWORD", true)
	}

	if p.Token.IsUnknown() {
		AddConfigurationAttributeError(diags, "token", "AAP_TOKEN", true)
	}

	if p.InsecureSkipVerify.IsUnknown() {
		AddConfigurationAttributeError(diags, "insecure_skip_verify", "AAP_INSECURE_SKIP_VERIFY", true)
	}

	if p.Timeout.IsUnknown() {
		AddConfigurationAttributeError(diags, "timeout", "AAP_TIMEOUT", true)
	}
}

const (
	DefaultTimeOut            = 5     // Default http session timeout
	DefaultInsecureSkipVerify = false // Default value for insecure skip verify
)

func (p *aapProviderModel) ReadValues(host, username, password *string, token *string, insecureSkipVerify *bool,
	timeout *int64, resp *provider.ConfigureResponse) {
	// Set default values from env variables

	// Prefer AAP_HOSTNAME, fallback to AAP_HOST
	*host = os.Getenv("AAP_HOSTNAME")
	if *host == "" {
		*host = os.Getenv("AAP_HOST")
	}

	// Read host from user configuration
	if !p.Host.IsNull() {
		*host = p.Host.ValueString()
	}

	*token = os.Getenv("AAP_TOKEN")
	if !p.Token.IsNull() {
		// Read token from user configuration
		*token = p.Token.ValueString()
	}

	if len(*token) > 0 {
		if !p.Username.IsNull() {
			resp.Diagnostics.AddAttributeWarning(
				path.Root("username"),
				"Inconsistent configuration for username",
				"When token is configured for authentication, username will be ignored. Please remove username from your configuration.",
			)
		}
		if !p.Password.IsNull() {
			resp.Diagnostics.AddAttributeWarning(
				path.Root("password"),
				"Inconsistent configuration for password",
				"When token is configured for authentication, password will be ignored. Please remove password from your configuration",
			)
		}
	} else {
		// Token not provided, proceed with username/password
		*username = os.Getenv("AAP_USERNAME")
		*password = os.Getenv("AAP_PASSWORD")

		// Read username from user configuration
		if !p.Username.IsNull() {
			*username = p.Username.ValueString()
		}
		// Read password from user configuration
		if !p.Password.IsNull() {
			*password = p.Password.ValueString()
		}
	}

	// setting default insecure skip verify value
	*insecureSkipVerify = DefaultInsecureSkipVerify
	var err error
	if !p.InsecureSkipVerify.IsNull() {
		*insecureSkipVerify = p.InsecureSkipVerify.ValueBool()
	} else if boolValue := os.Getenv("AAP_INSECURE_SKIP_VERIFY"); boolValue != "" {
		*insecureSkipVerify, err = strconv.ParseBool(boolValue)
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("insecure_skip_verify"),
				"Invalid value for insecure_skip_verify",
				"The provider cannot create the AAP API client as the value provided for insecure_skip_verify is not a valid boolean.",
			)
		}
	}

	// setting default timeout value
	*timeout = DefaultTimeOut
	if !p.Timeout.IsNull() {
		*timeout = p.Timeout.ValueInt64()
	} else if intValue := os.Getenv("AAP_TIMEOUT"); intValue != "" {
		// convert string into int64 value
		*timeout, err = strconv.ParseInt(intValue, 10, 64)
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("timeout"),
				"Invalid value for timeout",
				"The provider cannot create the AAP API client as the value provided for timeout is not a valid int64 value.",
			)
		}
	}
}
