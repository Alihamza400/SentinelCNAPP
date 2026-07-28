package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/golang-jwt/jwt/v5"
	"gopkg.in/yaml.v3"
)

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

const (
	RoleAdmin   = "admin"
	RoleViewer  = "viewer"
	RoleScanner = "scanner"

	ActionRead   = "read"
	ActionWrite  = "write"
	ActionDelete = "delete"
	ActionScan   = "scan"
	ActionAdmin  = "admin"
)

type Enforcer struct {
	enforcer *casbin.Enforcer
}

func NewEnforcer(policies []Policy) (*Enforcer, error) {
	m, err := model.NewModelFromString(RBACModel)
	if err != nil {
		return nil, fmt.Errorf("creating casbin model: %w", err)
	}

	e, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("creating casbin enforcer: %w", err)
	}

	for _, p := range policies {
		if _, err := e.AddPolicy(p.Subject, p.Object, p.Action); err != nil {
			return nil, fmt.Errorf("adding policy: %w", err)
		}
	}

	e.AddGroupingPolicy("viewer", "scanner")
	e.AddGroupingPolicy("scanner", "admin")

	return &Enforcer{enforcer: e}, nil
}

type Policy struct {
	Subject string `yaml:"subject"`
	Object  string `yaml:"object"`
	Action  string `yaml:"action"`
}

type PolicyFile struct {
	Policies []Policy `yaml:"policies"`
}

func LoadPoliciesFromYAML(data []byte) ([]Policy, error) {
	var pf PolicyFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("unmarshaling policies: %w", err)
	}
	return pf.Policies, nil
}

func DefaultPolicies() []Policy {
	return []Policy{
		{Subject: RoleAdmin, Object: "/*", Action: ".*"},
		{Subject: RoleViewer, Object: "/*", Action: "read"},
		{Subject: RoleScanner, Object: "/assets/*", Action: "read"},
		{Subject: RoleScanner, Object: "/findings", Action: "write"},
		{Subject: RoleScanner, Object: "/findings/*", Action: "write"},
		{Subject: RoleScanner, Object: "/scans", Action: "write"},
	}
}

func (e *Enforcer) Authorize(subject, object, action string) (bool, error) {
	return e.enforcer.Enforce(subject, object, action)
}

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

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Roles     []string  `json:"roles"`
	TeamID    string    `json:"team_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

type TokenClaims struct {
	Subject string   `json:"sub"`
	Email   string   `json:"email"`
	Name    string   `json:"name"`
	Roles   []string `json:"roles"`
	TeamID  string   `json:"team_id"`
	Exp     int64    `json:"exp"`
}

func (c *TokenClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	if c.Exp == 0 {
		return nil, nil
	}
	return jwt.NewNumericDate(time.Unix(c.Exp, 0)), nil
}

func (c *TokenClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return nil, nil
}

func (c *TokenClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return nil, nil
}

func (c *TokenClaims) GetIssuer() string {
	return ""
}

func (c *TokenClaims) GetSubject() string {
	return c.Subject
}

func (c *TokenClaims) GetAudience() jwt.ClaimStrings {
	return nil
}

type oidcConfigResponse struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Use string `json:"use"`
}

type jwksResponse struct {
	Keys []jwkKey `json:"keys"`
}

type OIDCValidator struct {
	issuerURL string
	clientID  string
	jwksURI   string

	mu          sync.RWMutex
	publicKeys  map[string]*rsa.PublicKey
	lastRefresh time.Time
	httpClient  *http.Client
}

var (
	defaultValidator     *OIDCValidator
	defaultValidatorMu   sync.Mutex
)

func NewOIDCValidator(issuerURL, clientID string) (*OIDCValidator, error) {
	v := &OIDCValidator{
		issuerURL:  strings.TrimRight(issuerURL, "/"),
		clientID:   clientID,
		publicKeys: make(map[string]*rsa.PublicKey),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	if err := v.refreshJWKS(); err != nil {
		return nil, fmt.Errorf("fetching OIDC configuration: %w", err)
	}

	return v, nil
}

func InitDefaultValidator(issuerURL, clientID string) (*OIDCValidator, error) {
	defaultValidatorMu.Lock()
	defer defaultValidatorMu.Unlock()

	v, err := NewOIDCValidator(issuerURL, clientID)
	if err != nil {
		return nil, err
	}

	defaultValidator = v
	return v, nil
}

func (v *OIDCValidator) refreshJWKS() error {
	if v.jwksURI == "" {
		wellKnown := v.issuerURL + "/.well-known/openid-configuration"
		req, err := http.NewRequestWithContext(context.Background(), "GET", wellKnown, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}

		resp, err := v.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("fetching OIDC config: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("OIDC config returned status %d", resp.StatusCode)
		}

		var oidcCfg oidcConfigResponse
		if err := json.NewDecoder(resp.Body).Decode(&oidcCfg); err != nil {
			return fmt.Errorf("decoding OIDC config: %w", err)
		}

		v.jwksURI = oidcCfg.JWKSURI
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", v.jwksURI, nil)
	if err != nil {
		return fmt.Errorf("creating JWKS request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS returned status %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decoding JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}

		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}

		n := new(big.Int).SetBytes(nBytes)
		e := int(new(big.Int).SetBytes(eBytes).Int64())

		keys[k.Kid] = &rsa.PublicKey{N: n, E: e}
	}

	v.mu.Lock()
	v.publicKeys = keys
	v.lastRefresh = time.Now()
	v.mu.Unlock()

	return nil
}

func (v *OIDCValidator) getKey(kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.publicKeys[kid]
	v.mu.RUnlock()

	if ok {
		return key, nil
	}

	if err := v.refreshJWKS(); err != nil {
		return nil, err
	}

	v.mu.RLock()
	key, ok = v.publicKeys[kid]
	v.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key %q not found in JWKS", kid)
	}

	return key, nil
}

func (v *OIDCValidator) ValidateToken(tokenString string) (*TokenClaims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("empty token")
	}

	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		kid, ok := t.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("token missing kid header")
		}

		return v.getKey(kid)
	},
		jwt.WithIssuer(v.issuerURL),
		jwt.WithAudience(v.clientID),
		jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	rawClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	claims := &TokenClaims{
		Subject: safeString(rawClaims["sub"]),
		Email:   safeString(rawClaims["email"]),
		Name:    safeString(rawClaims["name"]),
		TeamID:  safeString(rawClaims["team_id"]),
	}

	if exp, ok := rawClaims["exp"].(float64); ok {
		claims.Exp = int64(exp)
	}

	if rolesRaw, ok := rawClaims["roles"].([]interface{}); ok {
		claims.Roles = make([]string, 0, len(rolesRaw))
		for _, r := range rolesRaw {
			if s, ok := r.(string); ok {
				claims.Roles = append(claims.Roles, s)
			}
		}
	}

	if claims.Roles == nil {
		if groupsRaw, ok := rawClaims["groups"].([]interface{}); ok {
			claims.Roles = make([]string, 0, len(groupsRaw))
			for _, g := range groupsRaw {
				if s, ok := g.(string); ok {
					claims.Roles = append(claims.Roles, s)
				}
			}
		}
	}

	if claims.Roles == nil {
		claims.Roles = []string{RoleViewer}
	}

	return claims, nil
}

func safeString(v interface{}) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

var globalValidateToken = defaultValidateToken

func defaultValidateToken(tokenString string) (*TokenClaims, error) {
	defaultValidatorMu.Lock()
	v := defaultValidator
	defaultValidatorMu.Unlock()

	if v == nil {
		return nil, fmt.Errorf("OIDC validator not initialized: call InitDefaultValidator or set SENTINEL_OIDC_ISSUER_URL and SENTINEL_OIDC_CLIENT_ID")
	}

	return v.ValidateToken(tokenString)
}

func ValidateToken(tokenString string) (*TokenClaims, error) {
	return globalValidateToken(tokenString)
}

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

type contextKey string

const (
	UserKey    contextKey = "user"
	ClaimsKey  contextKey = "claims"
	SubjectKey contextKey = "subject"
)

func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserKey, user)
}

func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserKey).(*User)
	return user, ok
}
