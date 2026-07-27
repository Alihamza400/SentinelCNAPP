package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/casbin/v2/persist"
	"gopkg.in/yaml.v3"
)

// RBACModel is the Casbin RBAC model definition.
const RBACModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch(r.obj, p.obj) && regexMatch(r.act, p.act)
`

// Role constants.
const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"
	RoleScanner = "scanner"
)

// Permission constants.
const (
	ActionRead   = "read"
	ActionWrite  = "write"
	ActionDelete = "delete"
	ActionScan   = "scan"
	ActionAdmin  = "admin"
)

// Enforcer wraps Casbin for RBAC enforcement.
type Enforcer struct {
	enforcer *casbin.Enforcer
}

// NewEnforcer creates a new RBAC enforcer.
func NewEnforcer(policies []Policy) (*Enforcer, error) {
	m, err := model.NewModelFromString(RBACModel)
	if err != nil {
		return nil, fmt.Errorf("creating casbin model: %w", err)
	}

	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("creating casbin enforcer: %w", err)
	}

	// Add default role assignments
	for _, p := range policies {
		if _, err := e.AddPolicy(p.Subject, p.Object, p.Action); err != nil {
			return nil, fmt.Errorf("adding policy: %w", err)
		}
	}

	// Add role inheritance
	e.AddGroupingPolicy("viewer", "scanner")
	e.AddGroupingPolicy("scanner", "admin")

	return &Enforcer{enforcer: e}, nil
}

// Policy defines a Casbin policy rule.
type Policy struct {
	Subject string `yaml:"subject"` // role or user
	Object  string `yaml:"object"`  // resource pattern
	Action  string `yaml:"action"`  // action pattern
}

// PolicyFile defines the structure of a policy YAML file.
type PolicyFile struct {
	Policies []Policy `yaml:"policies"`
}

// LoadPoliciesFromYAML loads policies from a YAML file.
func LoadPoliciesFromYAML(data []byte) ([]Policy, error) {
	var pf PolicyFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("unmarshaling policies: %w", err)
	}
	return pf.Policies, nil
}

// DefaultPolicies returns the default RBAC policies.
func DefaultPolicies() []Policy {
	return []Policy{
		// Admin has full access
		{Subject: RoleAdmin, Object: "/*", Action: ".*"},
		// Viewer can read everything
		{Subject: RoleViewer, Object: "/*", Action: "read"},
		// Scanner can read assets and write findings
		{Subject: RoleScanner, Object: "/assets/*", Action: "read"},
		{Subject: RoleScanner, Object: "/findings", Action: "write"},
		{Subject: RoleScanner, Object: "/findings/*", Action: "write"},
		{Subject: RoleScanner, Object: "/scans", Action: "write"},
	}
}

// Authorize checks if a subject (user/role) is allowed to perform an action on an object.
func (e *Enforcer) Authorize(subject, object, action string) (bool, error) {
	return e.enforcer.Enforce(subject, object, action)
}

// Middleware returns an HTTP middleware that enforces RBAC.
func (e *Enforcer) Middleware(extractSubject func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject := extractSubject(r)
			if subject == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			object := r.URL.Path
			action := mapMethodToAction(r.Method)

			allowed, err := e.Authorize(subject, object, action)
			if err != nil || !allowed {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func mapMethodToAction(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return ActionRead
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return ActionWrite
	case http.MethodDelete:
		return ActionDelete
	default:
		return ActionRead
	}
}

// User represents an authenticated user.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Roles     []string  `json:"roles"`
	TeamID    string    `json:"team_id"`
	CreatedAt time.Time `json:"created_at"`
}

// HasRole checks if the user has a specific role.
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// TokenClaims represents JWT token claims.
type TokenClaims struct {
	Subject string   `json:"sub"`
	Email   string   `json:"email"`
	Name    string   `json:"name"`
	Roles   []string `json:"roles"`
	TeamID  string   `json:"team_id"`
	Exp     int64    `json:"exp"`
}

// ValidateToken validates a JWT token and returns the claims.
// This is a stub — production should use OIDC verification.
func ValidateToken(tokenString string) (*TokenClaims, error) {
	// TODO: Implement OIDC JWT validation
	// This is replaced with real OIDC verification in Phase 1
	if tokenString == "" {
		return nil, fmt.Errorf("empty token")
	}
	return nil, fmt.Errorf("oidc validation not yet implemented")
}

// ExtractSubjectFromBearer extracts the subject from a Bearer token in the Authorization header.
func ExtractSubjectFromBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	claims, err := ValidateToken(token)
	if err != nil {
		return ""
	}
	return claims.Subject
}

// Context keys for storing auth info in request context.
type contextKey string

const (
	UserKey    contextKey = "user"
	ClaimsKey  contextKey = "claims"
	SubjectKey contextKey = "subject"
)

// WithUser stores a user in context.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserKey, user)
}

// UserFromContext retrieves a user from context.
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserKey).(*User)
	return user, ok
}
