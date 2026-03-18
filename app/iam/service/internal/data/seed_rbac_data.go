package data

import (
	"github.com/Servora-Kit/servora/app/iam/service/internal/data/ent/rbacmenu"
)

// ─── Roles ───────────────────────────────────────────────────────────────────

type roleSpec struct {
	code        string
	name        string
	description string
}

var builtinRoles = []roleSpec{
	{"platform_admin", "平台管理员", "平台超级管理员，可管理所有租户"},
	{"tenant_owner", "租户所有者", "租户 Owner，可管理本租户的所有资源"},
	{"org_admin", "组织管理员", "组织 Admin，可管理组织成员"},
	{"org_member", "组织成员", "普通组织成员"},
}

// ─── Permission Groups ────────────────────────────────────────────────────────

type permGroupSpec struct {
	key    string
	name   string
	module string
}

var permGroupSpecs = []permGroupSpec{
	{"dashboard", "概览", "dashboard"},
	{"org", "组织管理", "organization"},
	{"position", "职位管理", "position"},
	{"app", "应用管理", "application"},
	{"user", "用户管理", "user"},
	{"member", "成员管理", "member"},
	{"tenant", "租户管理", "tenant"},
	{"rbac", "权限管理", "rbac"},
	{"system", "系统管理", "system"},
	{"settings", "个人设置", "settings"},
}

// ─── Permissions ──────────────────────────────────────────────────────────────

type permSpec struct {
	groupKey    string
	code        string
	name        string
	description string
}

var permSpecs = []permSpec{
	// Dashboard
	{"dashboard", "dashboard:view", "查看概览", "访问概览仪表盘"},
	{"dashboard", "dashboard:org_kpi", "查看组织 KPI", "仅查看当前组织的用户数量 KPI"},
	{"dashboard", "dashboard:full_kpi", "查看完整 KPI", "查看用户/组织/应用全量 KPI"},
	// Organization
	{"org", "org:list", "查看组织列表", "列出租户下的所有组织"},
	{"org", "org:create", "创建组织", "在租户下新建组织"},
	{"org", "org:manage", "管理组织", "编辑和删除组织"},
	// Position
	{"position", "position:list", "查看职位列表", "查看职位信息"},
	{"position", "position:create", "创建职位", "创建新职位"},
	{"position", "position:manage", "管理职位", "编辑和删除职位"},
	// Application
	{"app", "app:list", "查看应用列表", "列出租户下的所有应用"},
	{"app", "app:create", "创建应用", "在租户下注册新应用"},
	{"app", "app:manage", "管理应用", "编辑/删除应用及重新生成 ClientSecret"},
	// User
	{"user", "user:list", "查看用户列表", "列出租户下的所有用户"},
	{"user", "user:create", "创建用户", "在租户下创建新用户"},
	{"user", "user:manage", "管理用户", "编辑和删除用户"},
	// Member
	{"member", "member:view", "查看成员", "查看组织成员列表"},
	{"member", "member:manage", "管理成员", "添加/移除成员，修改成员角色"},
	// Tenant (platform admin only)
	{"tenant", "tenant:list", "查看租户列表", "平台管理员查看所有租户"},
	{"tenant", "tenant:create", "创建租户", "平台管理员创建新租户"},
	{"tenant", "tenant:manage", "管理租户", "平台管理员编辑/删除租户"},
	// RBAC management
	{"rbac", "rbac:role:list", "查看角色列表", "查看角色定义"},
	{"rbac", "rbac:role:manage", "管理角色", "创建/编辑/删除角色"},
	{"rbac", "rbac:permission:list", "查看权限列表", "查看权限码定义"},
	{"rbac", "rbac:permission:manage", "管理权限", "创建/编辑/删除权限码"},
	{"rbac", "rbac:menu:list", "查看菜单列表", "查看菜单树"},
	{"rbac", "rbac:menu:manage", "管理菜单", "创建/编辑/删除菜单项"},
	// System / Dict
	{"system", "dict:list", "查看字典", "查看系统字典"},
	{"system", "dict:manage", "管理字典", "创建/编辑/删除字典"},
	// Settings (accessed via header dropdown, no sidebar menu)
	{"settings", "settings:profile", "个人设置", "查看和编辑个人信息"},
	{"settings", "settings:security", "安全设置", "修改密码等安全设置"},
}

// ─── Menus ────────────────────────────────────────────────────────────────────

type menuSpec struct {
	key       string
	menuType  rbacmenu.Type
	name      string
	path      string
	component string
	icon      string
	sort      int
	parentKey string
}

// menuSpecs defines the full menu tree for the single-layout admin UI.
// Visibility is controlled entirely by permissionMenus mappings below.
// sort convention: 0=dashboard, 1=org, 2=user, 3=app, 4=rbac, 5=tenant, 6=system
var menuSpecs = []menuSpec{
	// ── Dashboard ──
	{"dashboard", rbacmenu.TypeMENU, "概览", "/dashboard", "dashboard", "LayoutDashboard", 0, ""},
	// ── Organization ──
	{"org_catalog", rbacmenu.TypeCATALOG, "组织管理", "", "", "Building2", 1, ""},
	{"org_list", rbacmenu.TypeMENU, "组织列表", "/organizations", "organizations/index", "FolderTree", 0, "org_catalog"},
	{"position_list", rbacmenu.TypeMENU, "职位管理", "/positions", "positions/index", "Briefcase", 1, "org_catalog"},
	// ── User ──
	{"user_catalog", rbacmenu.TypeCATALOG, "用户管理", "", "", "Users", 2, ""},
	{"user_list", rbacmenu.TypeMENU, "用户列表", "/users", "users/index", "User", 0, "user_catalog"},
	// ── Application ──
	{"app_catalog", rbacmenu.TypeCATALOG, "应用管理", "", "", "AppWindow", 3, ""},
	{"app_list", rbacmenu.TypeMENU, "应用列表", "/applications", "applications/index", "Layers", 0, "app_catalog"},
	// ── RBAC (role visible to tenant_owner; permissions/menus to platform_admin only) ──
	{"rbac_catalog", rbacmenu.TypeCATALOG, "权限管理", "", "", "ShieldCheck", 4, ""},
	{"rbac_roles", rbacmenu.TypeMENU, "角色管理", "/rbac/roles", "rbac/roles/index", "UserCog", 0, "rbac_catalog"},
	{"rbac_perms", rbacmenu.TypeMENU, "权限码管理", "/rbac/permissions", "rbac/permissions/index", "Key", 1, "rbac_catalog"},
	{"rbac_menus", rbacmenu.TypeMENU, "菜单管理", "/rbac/menus", "rbac/menus/index", "LayoutList", 2, "rbac_catalog"},
	// ── Tenant (platform_admin only) ──
	{"tenant_catalog", rbacmenu.TypeCATALOG, "租户管理", "", "", "Network", 5, ""},
	{"tenant_list", rbacmenu.TypeMENU, "租户列表", "/tenants", "tenants/index", "Store", 0, "tenant_catalog"},
	// ── System (platform_admin only) ──
	{"system_catalog", rbacmenu.TypeCATALOG, "系统管理", "", "", "Settings2", 6, ""},
	{"dict_list", rbacmenu.TypeMENU, "字典管理", "/system/dict", "system/dict/index", "BookOpen", 0, "system_catalog"},
}

// ─── Role → Permission mappings ───────────────────────────────────────────────

var rolePermissions = map[string][]string{
	"platform_admin": {
		"dashboard:view", "dashboard:full_kpi",
		"org:list", "org:create", "org:manage",
		"position:list", "position:create", "position:manage",
		"app:list", "app:create", "app:manage",
		"user:list", "user:create", "user:manage",
		"member:view", "member:manage",
		"tenant:list", "tenant:create", "tenant:manage",
		"rbac:role:list", "rbac:role:manage",
		"rbac:permission:list", "rbac:permission:manage",
		"rbac:menu:list", "rbac:menu:manage",
		"dict:list", "dict:manage",
		"settings:profile", "settings:security",
	},
	"tenant_owner": {
		"dashboard:view", "dashboard:full_kpi",
		"org:list", "org:create", "org:manage",
		"position:list", "position:create", "position:manage",
		"app:list", "app:create", "app:manage",
		"user:list", "user:create", "user:manage",
		"member:view", "member:manage",
		"rbac:role:list", "rbac:role:manage",
		"settings:profile", "settings:security",
	},
	"org_admin": {
		"dashboard:view", "dashboard:org_kpi",
		"org:list",
		"position:list",
		"member:view", "member:manage",
		"settings:profile", "settings:security",
	},
	"org_member": {
		"member:view",
		"settings:profile", "settings:security",
	},
}

// ─── Permission → Menu mappings ───────────────────────────────────────────────
//
// Visibility rule: a user sees a menu item if they have ANY permission that maps to it.
// Convention:
//   - list permissions grant the parent CATALOG + the leaf menu.
//   - manage-only permissions only need the leaf (catalog already granted by list).
//   - rbac:permission:list / rbac:menu:list only grant their own leaf because
//     rbac_catalog is already granted via rbac:role:list.

var permissionMenus = map[string][]string{
	"dashboard:view":         {"dashboard"},
	"dashboard:org_kpi":      {"dashboard"},
	"dashboard:full_kpi":     {"dashboard"},
	"org:list":               {"org_catalog", "org_list"},
	"org:create":             {"org_catalog", "org_list"},
	"org:manage":             {"org_list"},
	"position:list":          {"org_catalog", "position_list"},
	"position:create":        {"org_catalog", "position_list"},
	"position:manage":        {"position_list"},
	"app:list":               {"app_catalog", "app_list"},
	"app:create":             {"app_catalog", "app_list"},
	"app:manage":             {"app_list"},
	"user:list":              {"user_catalog", "user_list"},
	"user:create":            {"user_catalog", "user_list"},
	"user:manage":            {"user_list"},
	"member:view":            {"org_catalog", "org_list"},
	"member:manage":          {"org_catalog", "org_list"},
	"rbac:role:list":         {"rbac_catalog", "rbac_roles"},
	"rbac:role:manage":       {"rbac_catalog", "rbac_roles"},
	"rbac:permission:list":   {"rbac_perms"},
	"rbac:permission:manage": {"rbac_perms"},
	"rbac:menu:list":         {"rbac_menus"},
	"rbac:menu:manage":       {"rbac_menus"},
	"tenant:list":            {"tenant_catalog", "tenant_list"},
	"tenant:create":          {"tenant_catalog", "tenant_list"},
	"tenant:manage":          {"tenant_list"},
	"dict:list":              {"system_catalog", "dict_list"},
	"dict:manage":            {"system_catalog", "dict_list"},
	// settings:* are accessed via the header dropdown, not the sidebar
}

// ─── Permission → API mappings ────────────────────────────────────────────────

type apiSpec struct {
	permCode string
	method   string
	path     string
}

var permissionApis = []apiSpec{
	{"org:list", "GET", "/v1/organizations"},
	{"org:create", "POST", "/v1/organizations"},
	{"org:manage", "PUT", "/v1/organizations/{id}"},
	{"org:manage", "DELETE", "/v1/organizations/{id}"},
	{"position:list", "GET", "/v1/positions"},
	{"position:create", "POST", "/v1/positions"},
	{"position:manage", "PUT", "/v1/positions/{id}"},
	{"position:manage", "DELETE", "/v1/positions/{id}"},
	{"app:list", "GET", "/v1/applications"},
	{"app:create", "POST", "/v1/applications"},
	{"app:manage", "PUT", "/v1/applications/{id}"},
	{"app:manage", "DELETE", "/v1/applications/{id}"},
	{"app:manage", "POST", "/v1/applications/{id}/regenerate-secret"},
	{"user:list", "GET", "/v1/users"},
	{"user:create", "POST", "/v1/users"},
	{"user:manage", "PUT", "/v1/users/{id}"},
	{"user:manage", "DELETE", "/v1/users/{id}"},
	{"member:view", "GET", "/v1/organizations/{id}/members"},
	{"member:manage", "POST", "/v1/organizations/{id}/members"},
	{"member:manage", "PUT", "/v1/organizations/{id}/members/{userId}"},
	{"member:manage", "DELETE", "/v1/organizations/{id}/members/{userId}"},
	{"tenant:list", "GET", "/v1/tenants"},
	{"tenant:create", "POST", "/v1/tenants"},
	{"tenant:manage", "PUT", "/v1/tenants/{id}"},
	{"tenant:manage", "DELETE", "/v1/tenants/{id}"},
	{"dict:list", "GET", "/v1/dict/types"},
	{"dict:list", "GET", "/v1/dict/types/{code}/items"},
	{"dict:manage", "POST", "/v1/dict/types"},
	{"dict:manage", "PUT", "/v1/dict/types/{id}"},
	{"dict:manage", "DELETE", "/v1/dict/types/{id}"},
	{"dict:manage", "POST", "/v1/dict/types/{typeId}/items"},
	{"dict:manage", "PUT", "/v1/dict/types/{typeId}/items/{id}"},
	{"dict:manage", "DELETE", "/v1/dict/types/{typeId}/items/{id}"},
}

