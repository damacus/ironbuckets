package renderer

import (
	"html/template"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// TemplateRenderer implements echo.Renderer
type TemplateRenderer struct {
	Templates map[string]*template.Template
}

// New creates a new TemplateRenderer with pre-parsed templates
func New() *TemplateRenderer {
	r := &TemplateRenderer{
		Templates: make(map[string]*template.Template),
	}
	r.parseTemplates()
	return r
}

func (t *TemplateRenderer) parseTemplates() {
	// Helper to parse layout + page + confirm dialog partial
	parse := func(name, pageFile string) {
		t.Templates[name] = template.Must(template.ParseFiles(
			"views/layouts/base.html",
			"views/partials/confirm_dialog.html",
			"views/pages/"+pageFile,
		))
	}

	parse("dashboard", "dashboard.html")
	parse("drives", "drives.html")
	parse("users", "users.html")
	parse("groups", "groups.html")
	parse("group_detail", "group_detail.html")
	parse("buckets", "buckets.html")
	parse("browser", "browser.html")
	parse("settings", "settings.html")
	parse("bucket_settings", "bucket_settings.html")
	parse("policies", "policies.html")
	parse("service_accounts", "service_accounts.html")

	// Login is standalone
	t.Templates["login"] = template.Must(template.ParseFiles("views/pages/login.html"))
	// Error fragment
	t.Templates["login_error"] = template.Must(template.New("error").Parse(`{{.}}`))
	// Partials
	t.Templates["user_create_modal"] = template.Must(template.ParseFiles("views/partials/user_create_modal.html"))
	t.Templates["group_create_modal"] = template.Must(template.ParseFiles("views/partials/group_create_modal.html"))
	t.Templates["bucket_create_modal"] = template.Must(template.ParseFiles("views/partials/bucket_create_modal.html"))
	t.Templates["folder_create_modal"] = template.Must(template.ParseFiles("views/partials/folder_create_modal.html"))
	t.Templates["drives_widget"] = template.Must(template.ParseFiles("views/partials/drives_widget.html"))
	t.Templates["storage_widget"] = template.Must(template.ParseFiles("views/partials/storage_widget.html"))
	t.Templates["users_widget"] = template.Must(template.ParseFiles("views/partials/users_widget.html"))
	t.Templates["server_widget"] = template.Must(template.ParseFiles("views/partials/server_widget.html"))
	t.Templates["share_link"] = template.Must(template.ParseFiles("views/partials/share_link.html"))
	t.Templates["policy_modal"] = template.Must(template.ParseFiles("views/partials/policy_modal.html"))
	t.Templates["service_account_create_modal"] = template.Must(template.ParseFiles("views/partials/service_account_create_modal.html"))
	t.Templates["service_account_created"] = template.Must(template.ParseFiles("views/partials/service_account_created.html"))
	t.Templates["lifecycle_rules"] = template.Must(template.ParseFiles("views/partials/lifecycle_rules.html"))
	t.Templates["notifications"] = template.Must(template.ParseFiles("views/partials/notifications.html"))
	t.Templates["object_info"] = template.Must(template.ParseFiles("views/partials/object_info.html"))
	t.Templates["replication"] = template.Must(template.ParseFiles("views/partials/replication.html"))
	t.Templates["versioning_status"] = template.Must(template.ParseFiles("views/partials/versioning_status.html"))
	t.Templates["bucket_quota"] = template.Must(template.ParseFiles("views/partials/bucket_quota.html"))
	t.Templates["logs"] = template.Must(template.ParseFiles("views/partials/logs.html"))
	t.Templates["bucket_policy"] = template.Must(template.ParseFiles(
		"views/partials/bucket_policy.html",
		"views/partials/bucket_policy_badge.html",
	))
	t.Templates["bucket_policy_badge"] = template.Must(template.ParseFiles("views/partials/bucket_policy_badge.html"))
}

// selfExecutingTemplates lists templates that execute their own named block instead of "base"
var selfExecutingTemplates = map[string]bool{
	"user_create_modal":            true,
	"bucket_create_modal":          true,
	"folder_create_modal":          true,
	"group_create_modal":           true,
	"drives_widget":                true,
	"storage_widget":               true,
	"users_widget":                 true,
	"server_widget":                true,
	"share_link":                   true,
	"policy_modal":                 true,
	"service_account_create_modal": true,
	"service_account_created":      true,
	"lifecycle_rules":              true,
	"notifications":                true,
	"object_info":                  true,
	"replication":                  true,
	"versioning_status":            true,
	"bucket_quota":                 true,
	"logs":                         true,
	"bucket_policy":                true,
	"bucket_policy_badge":          true,
}

// Render renders a template document
func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	tmpl, ok := t.Templates[name]
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "Template not found: "+name)
	}

	// Templates that define their own named block execute that block directly
	if selfExecutingTemplates[name] {
		return tmpl.ExecuteTemplate(w, name, data)
	}
	// All other templates (pages with layout) execute the "base" block
	return tmpl.ExecuteTemplate(w, "base", data)
}
