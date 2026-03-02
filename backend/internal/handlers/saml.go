package handlers

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-syncflow/internal/models"
	"go-syncflow/internal/storage"
)

type samlProtocolConfig struct {
	SpEntityID   string `json:"spEntityId"`
	AcsURL       string `json:"acsUrl"`
	SloURL       string `json:"sloUrl"`
	NameIDFormat string `json:"nameIdFormat"`
	IdpEntityID  string `json:"idpEntityId"`
}

type samlAuthnRequest struct {
	XMLName                     xml.Name `xml:"AuthnRequest"`
	ID                          string   `xml:"ID,attr"`
	AssertionConsumerServiceURL string   `xml:"AssertionConsumerServiceURL,attr"`
	Issuer                      string   `xml:"Issuer"`
}

func SAMLMetadata(c *gin.Context) {
	app, cfg, err := getSAMLAppConfig(c.Param("appCode"))
	if err != nil {
		respondError(c, http.StatusNotFound, "应用不存在或未启用 SAML")
		return
	}
	if !app.Enabled {
		respondError(c, http.StatusForbidden, "应用已禁用")
		return
	}

	issuer := issuerURL(c)
	idpEntityID := cfg.IdpEntityID
	if idpEntityID == "" {
		idpEntityID = issuer + "/saml/" + app.Code
	}
	ssoURL := issuer + "/saml/" + app.Code + "/sso"

	_, certDER := getSAMLKeypair()
	certB64 := base64.StdEncoding.EncodeToString(certDER)

	metadata := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="` + xmlEscape(idpEntityID) + `">` + "\n" +
		`  <md:IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">` + "\n" +
		`    <md:KeyDescriptor use="signing">` + "\n" +
		`      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">` + "\n" +
		`        <ds:X509Data><ds:X509Certificate>` + certB64 + `</ds:X509Certificate></ds:X509Data>` + "\n" +
		`      </ds:KeyInfo>` + "\n" +
		`    </md:KeyDescriptor>` + "\n" +
		`    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="` + xmlEscape(ssoURL) + `"/>` + "\n" +
		`  </md:IDPSSODescriptor>` + "\n" +
		`</md:EntityDescriptor>`

	c.Header("Content-Type", "application/samlmetadata+xml; charset=utf-8")
	c.String(http.StatusOK, metadata)
}

func SAMLSso(c *gin.Context) {
	app, cfg, err := getSAMLAppConfig(c.Param("appCode"))
	if err != nil {
		respondError(c, http.StatusNotFound, "应用不存在或未启用 SAML")
		return
	}
	if !app.Enabled {
		respondError(c, http.StatusForbidden, "应用已禁用")
		return
	}
	if cfg.AcsURL == "" {
		respondError(c, http.StatusBadRequest, "未配置 ACS URL")
		return
	}

	samlReqB64 := strings.TrimSpace(c.PostForm("SAMLRequest"))
	relayState := strings.TrimSpace(c.PostForm("RelayState"))
	if samlReqB64 == "" {
		respondError(c, http.StatusBadRequest, "缺少 SAMLRequest")
		return
	}

	reqXML, err := base64.StdEncoding.DecodeString(samlReqB64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "SAMLRequest 无效")
		return
	}

	var req samlAuthnRequest
	_ = xml.Unmarshal(reqXML, &req)
	if req.AssertionConsumerServiceURL != "" && req.AssertionConsumerServiceURL != cfg.AcsURL {
		respondError(c, http.StatusBadRequest, "ACS 不匹配")
		return
	}
	if cfg.SpEntityID != "" && strings.TrimSpace(req.Issuer) != "" && strings.TrimSpace(req.Issuer) != cfg.SpEntityID {
		respondError(c, http.StatusBadRequest, "Issuer 不匹配")
		return
	}

	claims, err := parseBearerUser(c)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "未登录")
		return
	}
	if !canUserAccessApp(claims.UserID, app.ID, app.AccessMode) {
		respondError(c, http.StatusForbidden, "无权访问该应用")
		return
	}

	var user models.User
	if err := storage.DB.Preload("Roles").First(&user, claims.UserID).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}
	roles, perms := buildUserRBACForApp(app.ID, user)
	extraAttrs := loadSAMLExtraAttributes(app.ID)

	issuer := issuerURL(c)
	idpEntityID := cfg.IdpEntityID
	if idpEntityID == "" {
		idpEntityID = issuer + "/saml/" + app.Code
	}

	nameIDFormat := cfg.NameIDFormat
	if nameIDFormat == "" {
		nameIDFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
	}

	now := time.Now().UTC()
	notOnOrAfter := now.Add(5 * time.Minute)
	responseID := "R-" + randomURLSafeString(24)
	assertionID := "A-" + randomURLSafeString(24)

	samlResponse := buildSAMLResponseXML(idpEntityID, cfg.AcsURL, cfg.SpEntityID, req.ID, responseID, assertionID, now, notOnOrAfter, nameIDFormat, user.Username, user.Email, user.Phone, roles, perms, extraAttrs)
	samlRespB64 := base64.StdEncoding.EncodeToString([]byte(samlResponse))

	_ = recordSSOAudit(app.ID, "saml", user.ID, user.Username, "sso", true, "", c.ClientIP())

	html := buildSAMLPostFormHTML(cfg.AcsURL, samlRespB64, relayState)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

func getSAMLAppConfig(appCode string) (*models.SSOApp, *samlProtocolConfig, error) {
	var app models.SSOApp
	if err := storage.DB.Where("code = ?", appCode).First(&app).Error; err != nil {
		return nil, nil, err
	}
	var proto models.SSOAppProtocol
	if err := storage.DB.Where("app_id = ? AND protocol = ? AND enabled = ?", app.ID, "saml", true).First(&proto).Error; err != nil {
		return nil, nil, err
	}
	var cfg samlProtocolConfig
	if strings.TrimSpace(proto.Config) != "" {
		_ = json.Unmarshal([]byte(proto.Config), &cfg)
	}
	return &app, &cfg, nil
}

func buildSAMLResponseXML(issuer, acsURL, audience, inResponseTo, responseID, assertionID string, now, notOnOrAfter time.Time, nameIDFormat, username, email, phone string, roles, perms []string, extraAttrs map[string][]string) string {
	if audience == "" {
		audience = "urn:app:" + issuer
	}
	issueInstant := now.Format(time.RFC3339Nano)
	notBefore := now.Add(-30 * time.Second).Format(time.RFC3339Nano)
	notAfter := notOnOrAfter.Format(time.RFC3339Nano)

	attrs := ""
	attrs += samlAttribute("roles", roles)
	attrs += samlAttribute("permissions", perms)
	if email != "" {
		attrs += samlAttribute("email", []string{email})
	}
	if phone != "" {
		attrs += samlAttribute("phone", []string{phone})
	}
	for k, v := range extraAttrs {
		if strings.TrimSpace(k) == "" || len(v) == 0 {
			continue
		}
		attrs += samlAttribute(k, v)
	}

	return `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="` + xmlEscape(responseID) + `" Version="2.0" IssueInstant="` + issueInstant + `" Destination="` + xmlEscape(acsURL) + `" InResponseTo="` + xmlEscape(inResponseTo) + `">` + "\n" +
		`  <saml:Issuer>` + xmlEscape(issuer) + `</saml:Issuer>` + "\n" +
		`  <samlp:Status><samlp:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/></samlp:Status>` + "\n" +
		`  <saml:Assertion ID="` + xmlEscape(assertionID) + `" Version="2.0" IssueInstant="` + issueInstant + `">` + "\n" +
		`    <saml:Issuer>` + xmlEscape(issuer) + `</saml:Issuer>` + "\n" +
		`    <saml:Subject>` + "\n" +
		`      <saml:NameID Format="` + xmlEscape(nameIDFormat) + `">` + xmlEscape(username) + `</saml:NameID>` + "\n" +
		`      <saml:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer">` + "\n" +
		`        <saml:SubjectConfirmationData InResponseTo="` + xmlEscape(inResponseTo) + `" NotOnOrAfter="` + notAfter + `" Recipient="` + xmlEscape(acsURL) + `"/>` + "\n" +
		`      </saml:SubjectConfirmation>` + "\n" +
		`    </saml:Subject>` + "\n" +
		`    <saml:Conditions NotBefore="` + notBefore + `" NotOnOrAfter="` + notAfter + `">` + "\n" +
		`      <saml:AudienceRestriction><saml:Audience>` + xmlEscape(audience) + `</saml:Audience></saml:AudienceRestriction>` + "\n" +
		`    </saml:Conditions>` + "\n" +
		`    <saml:AuthnStatement AuthnInstant="` + issueInstant + `" SessionIndex="` + xmlEscape(assertionID) + `">` + "\n" +
		`      <saml:AuthnContext><saml:AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:PasswordProtectedTransport</saml:AuthnContextClassRef></saml:AuthnContext>` + "\n" +
		`    </saml:AuthnStatement>` + "\n" +
		`    <saml:AttributeStatement>` + "\n" +
		attrs +
		`    </saml:AttributeStatement>` + "\n" +
		`  </saml:Assertion>` + "\n" +
		`</samlp:Response>`
}

func loadSAMLExtraAttributes(appID uint) map[string][]string {
	var tpl models.SSOAppTemplate
	if err := storage.DB.Where("app_id = ? AND protocol = ? AND enabled = ?", appID, "saml", true).Order("id DESC").First(&tpl).Error; err != nil {
		return map[string][]string{}
	}
	if strings.TrimSpace(tpl.Template) == "" {
		return map[string][]string{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(tpl.Template), &m); err != nil {
		return map[string][]string{}
	}
	rawAttrs, ok := m["extraAttributes"]
	if !ok {
		rawAttrs = m["attributes"]
	}
	attrsObj, ok := rawAttrs.(map[string]any)
	if !ok {
		return map[string][]string{}
	}
	out := map[string][]string{}
	for name, raw := range attrsObj {
		switch v := raw.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				out[name] = []string{v}
			}
		case []any:
			var vals []string
			for _, x := range v {
				if s, ok := x.(string); ok && strings.TrimSpace(s) != "" {
					vals = append(vals, s)
				}
			}
			if len(vals) > 0 {
				out[name] = vals
			}
		}
	}
	return out
}

func samlAttribute(name string, values []string) string {
	if len(values) == 0 {
		return ""
	}
	out := `      <saml:Attribute Name="` + xmlEscape(name) + `">` + "\n"
	for _, v := range values {
		out += `        <saml:AttributeValue xsi:type="xs:string" xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` + xmlEscape(v) + `</saml:AttributeValue>` + "\n"
	}
	out += `      </saml:Attribute>` + "\n"
	return out
}

func buildSAMLPostFormHTML(acsURL string, samlResponseB64 string, relayState string) string {
	rs := ""
	if relayState != "" {
		rs = `<input type="hidden" name="RelayState" value="` + htmlEscape(relayState) + `"/>`
	}
	return `<!doctype html><html><head><meta charset="utf-8"><title>SAML Redirect</title></head><body>` +
		`<form id="samlForm" method="post" action="` + htmlEscape(acsURL) + `">` +
		`<input type="hidden" name="SAMLResponse" value="` + htmlEscape(samlResponseB64) + `"/>` +
		rs +
		`</form>` +
		`<script>document.getElementById('samlForm').submit();</script>` +
		`</body></html>`
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}
