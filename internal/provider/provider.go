package provider

import (
	"context"
	"fmt"
	"github.com/devoteamgcloud/terraform-provider-looker/pkg/lookergo"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	defaultTimeout     = 5 * time.Minute
	minimumRefreshWait = 3 * time.Second
	checkDelay         = 10 * time.Second
)

func init() {
	// Set descriptions to support markdown syntax, this will be used in document generation
	// and the language server.
	schema.DescriptionKind = schema.StringMarkdown

	// Customize the content of descriptions when output. For example you can add defaults on
	// to the exported descriptions if present.
	// schema.SchemaDescriptionBuilder = func(s *schema.Schema) string {
	// 	desc := s.Description
	// 	if s.Default != nil {
	// 		desc += fmt.Sprintf(" Defaults to `%v`.", s.Default)
	// 	}
	// 	return strings.TrimSpace(desc)
	// }
	//   /**/-
}

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: map[string]*schema.Schema{
				"base_url": &schema.Schema{
					Description: "For base_url, provide the URL including /api/ ! " +
						"Normally, a REST API should not have api in it's path, " +
						"therefore we don't add the /api/ inside the provider. ",
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("LOOKER_BASE_URL", nil),
				},
				"client_id": &schema.Schema{
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("LOOKER_API_CLIENT_ID", nil),
				},
				"client_secret": &schema.Schema{
					Type:        schema.TypeString,
					Optional:    true,
					Sensitive:   true,
					DefaultFunc: schema.EnvDefaultFunc("LOOKER_API_CLIENT_SECRET", nil),
				},
			},
			DataSourcesMap: map[string]*schema.Resource{
				"looker_user":           dataSourceUser(),
				"looker_group":          dataSourceGroup(),
				"looker_permission_set": {},
			},
			ResourcesMap: map[string]*schema.Resource{
				"looker_user":                   resourceUser(),
				"looker_group":                  resourceGroup(),
				"looker_group_member":           resourceGroupMember(),
				"looker_role":                   {},
				"looker_role_member":            {},
				"looker_connection":             resourceConnection(),
				"looker_project":                resourceProject(),
				"looker_project_git_deploy_key": {},
				"looker_project_git_repo":       {},
				"looker_lookml_model":           {},
				"looker_model_set":              {},
			},
		}

		p.ConfigureContextFunc = func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
			return providerConfigure(ctx, d, p, version)
		}

		return p
	}
}

type Workspace int

const (
	WorkspaceProduction Workspace = iota
	WorkspaceDev
)

type Config struct {
	Api       *lookergo.Client
	Workspace Workspace
}

func providerConfigure(ctx context.Context, d *schema.ResourceData, p *schema.Provider, version string) (interface{}, diag.Diagnostics) {
	tflog.Debug(ctx, "Configure provider", map[string]interface{}{"conninfo": d.ConnInfo(), "schema": p.Schema})
	tflog.Debug(ctx, "Provider config", map[string]interface{}{"client_id": d.Get("client_id").(string)})

	userAgent := p.UserAgent("terraform-provider-looker", version)
	var diags diag.Diagnostics

	client := lookergo.NewClient(nil)

	if err := client.SetBaseURL(d.Get("base_url").(string)); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to set looker API endpoint",
			Detail:   "Err: " + err.Error(),
		})
		return nil, diags
	}
	clientId := d.Get("client_id").(string)
	clientSecret := d.Get("client_secret").(string)
	if err := client.SetOauthCredentials(ctx, clientId, clientSecret); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to set looker API client_id/client_secret",
			Detail:   "Err: " + err.Error(),
		})
		return nil, diags
	}
	if err := client.SetUserAgent(userAgent); err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to set looker API user-agent",
			Detail:   "Err: " + err.Error(),
		})
		return nil, diags
	}

	session, _, err := client.Sessions.Get(ctx)
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to create Looker client",
			Detail:   "Unable to authenticate user for authenticated Looker client: " + err.Error(),
		})
		return nil, diags
	}

	var config Config

	switch session.WorkspaceId {
	case "production":
		config = Config{Api: client, Workspace: WorkspaceProduction}
	case "dev":
		config = Config{Api: client, Workspace: WorkspaceDev}
	default:
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Session Workspace ID is nome of production/dev",
			Detail:   "Unable to find workspace ID in /session call",
		})
		return nil, diags
	}

	devClient, devSession, err := client.CreateDevConnection(ctx)
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to create Looker client",
			Detail:   "Unable to authenticate user for authenticated Looker client: " + err.Error(),
		})
		return nil, diags
	}

	fmt.Printf("dc: %#v\nds: %#v\n", devClient, devSession)

	return &config, nil
}

/*
	baseUrl, _ := url.Parse(d.Get("base_url").(string))
	clientId := d.Get("client_id").(string)
	clientSecret := d.Get("client_secret").(string)

	client := lookergo.NewFromApiv3Creds(lookergo.ApiConfig{
		ClientId:     clientId,
		ClientSecret: clientSecret,
		BaseURL:      d.Get("base_url").(string),
		ClientCtx:    ctx,
		// ClientCtx:    context.Background(),
	})
	client.BaseURL = baseUrl
	client.UserAgent = userAgent

	sess, _, err := client.Sessions.Get(ctx)
	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to create Looker client",
			Detail:   "Unable to authenticate user for authenticated Looker client: " + err.Error(),
		})

		return nil, diags
	}

	if sess.WorkspaceId == "" {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Session Workspace ID is empty",
			Detail:   "Unable to find workspace ID in /session call",
		})

		return nil, diags
	}

	config := Config{Api: client}
	return &config, nil
*/

/*

// configure  -
func configure(version string, p *schema.Provider) func(context.Context, *schema.ResourceData) (interface{}, diag.Diagnostics) {
	return func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		tflog.Info(ctx, "Configure provider", map[string]interface{}{"conninfo": d.ConnInfo()})

		userAgent := p.UserAgent("terraform-provider-looker", version)
		var diags diag.Diagnostics

		baseUrl, _ := url.Parse(d.Get("base_url").(string))
		clientId := d.Get("client_id").(string)
		clientSecret := d.Get("client_secret").(string)

		client := lookergo.NewFromApiv3Creds(lookergo.ApiConfig{
			ClientId:     clientId,
			ClientSecret: clientSecret,
			BaseURL:      d.Get("base_url").(string),
		})
		client.BaseURL = baseUrl
		client.UserAgent = userAgent

		sess, _, err := client.Sessions.Get(ctx)
		if err != nil {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Unable to create Looker client",
				Detail:   "Unable to authenticate user for authenticated Looker client: " + err.Error(),
			})

			return nil, diags
		}

		if sess.WorkspaceId == "" {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Session Workspace ID is empty",
				Detail:   "Unable to find workspace ID in /session call",
			})

			return nil, diags
		}

		return client, nil
	}
}
*/
