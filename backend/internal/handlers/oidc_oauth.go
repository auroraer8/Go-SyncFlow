package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"go-syncflow/internal/middleware"
	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

type oidcClientConfig struct {
	ClientID          string   `json:"clientId"`
	ClientSecretHash  string   `json:"clientSecretHash"`
	RedirectURIs      []string `json:"redirectUris"`
	AccessTokenTTLSec int      `json:"accessTokenTtlSec"`
	IDTokenTTLSec     int      `json:"idTokenTtlSec"`
}

func OIDCDiscovery(c *gin.Context) {
	issuer := issuerURL(c)
	cfg := gin.H{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/oauth/authorize",
		"token_endpoint":                        issuer + "/oauth/token",
		"userinfo_endpoint":                     issuer + "/oidc/userinfo",
		"jwks_uri":                              issuer + "/oidc/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "profile", "email", "phone", "roles", "permissions"},
		"claims_supported":                      []string{"sub", "preferred_username", "email", "phone", "roles", "permissions"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
	}
	c.JSON(http.StatusOK, cfg)
}

func OIDCJWKS(c *gin.Context) {
	_, pubDER, kid := getOIDCSigningKey()
	pub, err := x509ParsePKIXPublicKey(pubDER)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "密钥不可用")
		return
	}
	n := base64url(pub.N.Bytes())
	e := base64url(bigIntToBytes(pub.E))
	c.JSON(http.StatusOK, gin.H{
		"keys": []gin.H{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": kid,
				"n":   n,
				"e":   e,
			},
		},
	})
}

func OAuthAuthorize(c *gin.Context) {
	clientID := strings.TrimSpace(c.Query("client_id"))
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	responseType := strings.TrimSpace(c.Query("response_type"))
	scope := strings.TrimSpace(c.Query("scope"))
	state := strings.TrimSpace(c.Query("state"))
	nonce := strings.TrimSpace(c.Query("nonce"))
	codeChallenge := strings.TrimSpace(c.Query("code_challenge"))
	codeChallengeMethod := strings.TrimSpace(c.Query("code_challenge_method"))

	if responseType != "code" || clientID == "" || redirectURI == "" {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}
	if codeChallenge == "" || strings.ToUpper(codeChallengeMethod) != "S256" {
		respondError(c, http.StatusBadRequest, "必须使用 PKCE(S256)")
		return
	}

	app, cfg, err := findOIDCAppByClientID(clientID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "client_id 无效")
		return
	}
	if !app.Enabled {
		respondError(c, http.StatusForbidden, "应用已禁用")
		return
	}
	if !isRedirectAllowed(cfg.RedirectURIs, redirectURI) {
		respondError(c, http.StatusBadRequest, "redirect_uri 未授权")
		return
	}

	claims, err := parseBearerUser(c)
	if err != nil {
		redirectToLogin(c)
		return
	}
	if !canUserAccessApp(claims.UserID, app.ID, app.AccessMode) {
		respondError(c, http.StatusForbidden, "无权访问该应用")
		return
	}

	code := randomURLSafeString(32)
	codeHash := sha256Hex(code)
	expiresAt := time.Now().Add(3 * time.Minute)
	row := models.SSOOAuthCode{
		CodeHash:            codeHash,
		AppID:               app.ID,
		ClientID:            clientID,
		UserID:              claims.UserID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		State:               state,
		Nonce:               nonce,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: "S256",
		ExpiresAt:           expiresAt,
		CreatedAt:           time.Now(),
	}
	if err := storage.DB.Create(&row).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "生成授权码失败")
		return
	}

	_ = recordSSOAudit(app.ID, "oidc", claims.UserID, claims.Username, "authorize", true, "", c.ClientIP())

	u, _ := url.Parse(redirectURI)
	q := u.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusFound, u.String())
}

func OAuthToken(c *gin.Context) {
	grantType := strings.TrimSpace(c.PostForm("grant_type"))
	code := strings.TrimSpace(c.PostForm("code"))
	redirectURI := strings.TrimSpace(c.PostForm("redirect_uri"))
	clientID := strings.TrimSpace(c.PostForm("client_id"))
	clientSecret := strings.TrimSpace(c.PostForm("client_secret"))
	codeVerifier := strings.TrimSpace(c.PostForm("code_verifier"))

	if grantType != "authorization_code" || code == "" || redirectURI == "" || clientID == "" || codeVerifier == "" {
		respondError(c, http.StatusBadRequest, "参数错误")
		return
	}

	app, cfg, err := findOIDCAppByClientID(clientID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "client_id 无效")
		return
	}
	if cfg.ClientSecretHash != "" && sha256Hex(clientSecret) != cfg.ClientSecretHash {
		respondError(c, http.StatusUnauthorized, "client_secret 无效")
		return
	}

	var authCode models.SSOOAuthCode
	if err := storage.DB.Where("code_hash = ?", sha256Hex(code)).First(&authCode).Error; err != nil {
		respondError(c, http.StatusBadRequest, "code 无效")
		return
	}
	if authCode.UsedAt != nil || time.Now().After(authCode.ExpiresAt) {
		respondError(c, http.StatusBadRequest, "code 已过期")
		return
	}
	if authCode.ClientID != clientID || authCode.RedirectURI != redirectURI {
		respondError(c, http.StatusBadRequest, "code 无效")
		return
	}

	if base64url(sha256Bytes(codeVerifier)) != authCode.CodeChallenge {
		respondError(c, http.StatusBadRequest, "code_verifier 无效")
		return
	}

	now := time.Now()
	storage.DB.Model(&authCode).Update("used_at", now)

	var user models.User
	if err := storage.DB.Preload("Roles").First(&user, authCode.UserID).Error; err != nil {
		respondError(c, http.StatusBadRequest, "用户不存在")
		return
	}

	roles, perms := buildUserRBACForApp(app.ID, user)
	granted := loadAppGrantedEntitlements(app.ID, user)
	effectiveScope := mergeScopes(authCode.Scope, granted.Scopes)

	accessTTL := time.Hour
	if cfg.AccessTokenTTLSec > 0 {
		accessTTL = time.Duration(cfg.AccessTokenTTLSec) * time.Second
	}
	idTTL := time.Hour
	if cfg.IDTokenTTLSec > 0 {
		idTTL = time.Duration(cfg.IDTokenTTLSec) * time.Second
	}

	issuer := issuerURL(c)
	accessToken, err := signOIDCJWT(issuer, clientID, user, roles, perms, now.Add(accessTTL), false, effectiveScope)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "签发失败")
		return
	}
	idToken, err := signOIDCJWT(issuer, clientID, user, roles, perms, now.Add(idTTL), true, effectiveScope)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "签发失败")
		return
	}

	_ = recordSSOAudit(app.ID, "oidc", user.ID, user.Username, "token", true, "", c.ClientIP())

	c.JSON(http.StatusOK, gin.H{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(accessTTL.Seconds()),
		"scope":        effectiveScope,
		"id_token":     idToken,
	})
}

func OIDCUserInfo(c *gin.Context) {
	tokenStr := extractBearerToken(c)
	if tokenStr == "" {
		respondError(c, http.StatusUnauthorized, "未登录")
		return
	}
	claims, err := parseOIDCAccessToken(tokenStr)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "token 无效")
		return
	}
	c.JSON(http.StatusOK, claims)
}

func issuerURL(c *gin.Context) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + c.Request.Host
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func sha256Bytes(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func mergeScopes(scope string, extra []string) string {
	set := map[string]struct{}{}
	for _, s := range strings.Fields(scope) {
		set[s] = struct{}{}
	}
	for _, s := range extra {
		s = strings.TrimSpace(s)
		if s != "" {
			set[s] = struct{}{}
		}
	}
	var out []string
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

func extractBearerToken(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if h == "" {
		if t, err := c.Cookie("gsf_token"); err == nil && t != "" {
			return t
		}
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return parts[1]
}

func redirectToLogin(c *gin.Context) {
	target := c.Request.URL.RequestURI()
	c.Redirect(http.StatusFound, "/login?redirect="+url.QueryEscape(target))
}

func parseBearerUser(c *gin.Context) (*middleware.Claims, error) {
	tokenStr := extractBearerToken(c)
	if tokenStr == "" {
		return nil, jwt.ErrTokenUnverifiable
	}
	return middleware.ParseToken(tokenStr)
}

func findOIDCAppByClientID(clientID string) (*models.SSOApp, *oidcClientConfig, error) {
	var protocols []models.SSOAppProtocol
	if err := storage.DB.Where("protocol = ? AND enabled = ?", "oidc", true).Find(&protocols).Error; err != nil {
		return nil, nil, err
	}
	for _, p := range protocols {
		var cfg oidcClientConfig
		if strings.TrimSpace(p.Config) != "" {
			_ = json.Unmarshal([]byte(p.Config), &cfg)
		}
		var app models.SSOApp
		if err := storage.DB.First(&app, p.AppID).Error; err != nil {
			continue
		}
		effectiveClientID := cfg.ClientID
		if effectiveClientID == "" {
			effectiveClientID = app.Code
		}
		if effectiveClientID == clientID {
			return &app, &cfg, nil
		}
	}
	return nil, nil, errors.New("not found")
}

func isRedirectAllowed(allowed []string, redirectURI string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, u := range allowed {
		if strings.TrimSpace(u) == redirectURI {
			return true
		}
	}
	return false
}

func canUserAccessApp(userID uint, appID uint, accessMode string) bool {
	if accessMode == "" || accessMode == "all" {
		return true
	}
	var user models.User
	if err := storage.DB.Preload("Roles").First(&user, userID).Error; err != nil {
		return false
	}

	switch accessMode {
	case "allowlist":
		var count int64
		storage.DB.Model(&models.SSOAppMapping{}).
			Where("app_id = ? AND type = ? AND (source = ? OR source = ?)", appID, "allowlist", user.Username, "user:"+user.Username).
			Count(&count)
		if count > 0 {
			return true
		}
		storage.DB.Model(&models.SSOAppMapping{}).
			Where("app_id = ? AND type = ? AND source = ?", appID, "allowlist", "uid:"+strconv.FormatUint(uint64(user.ID), 10)).
			Count(&count)
		return count > 0
	case "role_based":
		var roleMappings []models.SSOAppMapping
		if err := storage.DB.Where("app_id = ? AND type = ?", appID, "role").Find(&roleMappings).Error; err != nil {
			return false
		}
		if len(roleMappings) == 0 {
			return true
		}
		roleSet := map[string]struct{}{}
		for _, r := range user.Roles {
			roleSet[r.Code] = struct{}{}
			roleSet["role:"+r.Code] = struct{}{}
		}
		for _, m := range roleMappings {
			if _, ok := roleSet[m.Source]; ok {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func buildUserRBACForApp(appID uint, user models.User) ([]string, []string) {
	var roleCodes []string
	for _, r := range user.Roles {
		roleCodes = append(roleCodes, r.Code)
	}
	permCodes := middleware.GetUserPermissions(user.ID)

	var mappings []models.SSOAppMapping
	_ = storage.DB.Where("app_id = ?", appID).Find(&mappings).Error

	mappedRoles := map[string]struct{}{}
	mappedPerms := map[string]struct{}{}
	for _, m := range mappings {
		switch m.Type {
		case "role":
			for _, rc := range roleCodes {
				if m.Source == rc || m.Source == "role:"+rc {
					if m.Target != "" {
						mappedRoles[m.Target] = struct{}{}
					}
				}
			}
		case "permission":
			for _, pc := range permCodes {
				if m.Source == pc || m.Source == "permission:"+pc {
					if m.Target != "" {
						mappedPerms[m.Target] = struct{}{}
					}
				}
			}
		}
	}
	if len(mappedRoles) == 0 {
		for _, rc := range roleCodes {
			mappedRoles[rc] = struct{}{}
		}
	}
	if len(mappedPerms) == 0 {
		for _, pc := range permCodes {
			mappedPerms[pc] = struct{}{}
		}
	}

	granted := loadAppGrantedEntitlements(appID, user)
	for _, v := range granted.Roles {
		if v != "" {
			mappedRoles[v] = struct{}{}
		}
	}
	for _, v := range granted.Permissions {
		if v != "" {
			mappedPerms[v] = struct{}{}
		}
	}

	var roles []string
	for k := range mappedRoles {
		roles = append(roles, k)
	}
	var perms []string
	for k := range mappedPerms {
		perms = append(perms, k)
	}
	return roles, perms
}

type appGrantedEntitlements struct {
	Roles       []string
	Permissions []string
	Scopes      []string
}

func loadAppGrantedEntitlements(appID uint, user models.User) appGrantedEntitlements {
	var roleIDs []uint
	for _, r := range user.Roles {
		if r.ID != 0 {
			roleIDs = append(roleIDs, r.ID)
		}
	}

	q := storage.DB.Model(&models.SSOAppEntitlement{}).
		Select("sso_app_entitlements.*").
		Joins("JOIN sso_app_grant_items gi ON gi.entitlement_id = sso_app_entitlements.id").
		Joins("JOIN sso_app_grants g ON g.id = gi.grant_id").
		Where("g.app_id = ? AND g.enabled = ? AND sso_app_entitlements.enabled = ?", appID, true, true)

	if len(roleIDs) > 0 {
		q = q.Where("(g.subject_type = ? AND g.subject_id = ?) OR (g.subject_type = ? AND g.subject_id IN ?)", "user", user.ID, "role", roleIDs)
	} else {
		q = q.Where("g.subject_type = ? AND g.subject_id = ?", "user", user.ID)
	}

	var ents []models.SSOAppEntitlement
	_ = q.Find(&ents).Error

	out := appGrantedEntitlements{}
	roleSet := map[string]struct{}{}
	permSet := map[string]struct{}{}
	scopeSet := map[string]struct{}{}
	for _, e := range ents {
		switch e.Type {
		case "role":
			roleSet[e.Value] = struct{}{}
		case "permission":
			permSet[e.Value] = struct{}{}
		case "scope":
			scopeSet[e.Value] = struct{}{}
		}
	}
	for k := range roleSet {
		out.Roles = append(out.Roles, k)
	}
	for k := range permSet {
		out.Permissions = append(out.Permissions, k)
	}
	for k := range scopeSet {
		out.Scopes = append(out.Scopes, k)
	}
	return out
}

func signOIDCJWT(issuer, aud string, user models.User, roles, perms []string, exp time.Time, isIDToken bool, scope string) (string, error) {
	key, _, kid := getOIDCSigningKey()

	sub := user.Username
	claims := jwt.MapClaims{
		"iss":                issuer,
		"sub":                sub,
		"aud":                aud,
		"exp":                exp.Unix(),
		"iat":                time.Now().Unix(),
		"preferred_username": user.Username,
		"roles":              roles,
		"permissions":        perms,
		"scope":              scope,
	}
	if user.Email != "" {
		claims["email"] = user.Email
	}
	if user.Phone != "" {
		claims["phone"] = user.Phone
	}
	if isIDToken {
		claims["typ"] = "id_token"
	} else {
		claims["typ"] = "access_token"
	}

	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	t.Header["kid"] = kid
	return t.SignedString(key)
}

func parseOIDCAccessToken(tokenStr string) (jwt.MapClaims, error) {
	_, pubDER, _ := getOIDCSigningKey()
	pub, err := x509ParsePKIXPublicKey(pubDER)
	if err != nil {
		return nil, err
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return pub, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}
	return nil, jwt.ErrTokenUnverifiable
}

func recordSSOAudit(appID uint, protocol string, userID uint, username string, action string, success bool, message string, clientIP string) error {
	row := models.SSOAuditLog{
		AppID:     appID,
		Protocol:  protocol,
		UserID:    userID,
		Username:  username,
		Action:    action,
		Success:   success,
		Message:   message,
		ClientIP:  clientIP,
		CreatedAt: time.Now(),
	}
	return storage.DB.Create(&row).Error
}

func randomURLSafeString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64url(b)
}

func x509ParsePKIXPublicKey(pubDER []byte) (*rsa.PublicKey, error) {
	pubAny, err := x509.ParsePKIXPublicKey(pubDER)
	if err != nil {
		return nil, err
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not rsa public key")
	}
	return pub, nil
}

func bigIntToBytes(v int) []byte {
	if v == 0 {
		return []byte{0}
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte(v & 0xff)}, b...)
		v >>= 8
	}
	return b
}
