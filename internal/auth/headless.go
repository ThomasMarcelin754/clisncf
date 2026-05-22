package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/enetx/g"
	"github.com/enetx/surf"
)

const (
	auth0Domain  = "https://auth.monidentifiant.sncf"
	publicClient = "2bXa1Zw4AZHBZ1JZ2QjuYcYgano0ZZGr"
	bffBase      = "https://www.sncf-connect.com/bff"
	bffKey       = "ah1MPO-izehIHD-QZZ9y88n-kku876"
	webRedirect  = "https://www.sncf-connect.com/authenticate"
)

// HeadlessLogin performs the full headless OIDC login:
//  1. BFF /authenticate → Auth0 URL with BFF's PKCE
//  2. /co/authenticate → login_ticket
//  3. /authorize + login_ticket → MFA detect → OTP email
//  4. OTP submission → authorization code
//  5. /authenticate/apply → tokens
//
// otpFunc is called when the user needs to enter the OTP code.
// It receives the masked email (e.g. "thom***@outl***") and returns the 6-digit code.
func HeadlessLogin(email, password string, otpFunc func(maskedEmail string) (string, error)) (*Session, error) {
	client := newHeadlessSurfClient()
	defer func() { _ = client.Close() }()

	state := headlessUUID4()

	auth0URL, err := bffAuthenticate(client, state)
	if err != nil {
		return nil, fmt.Errorf("BFF /authenticate: %w", err)
	}

	loginTicket, err := coAuthenticate(client, email, password)
	if err != nil {
		return nil, fmt.Errorf("/co/authenticate: %w", err)
	}

	code, err := authorizeWithMFA(client, auth0URL, loginTicket, otpFunc)
	if err != nil {
		return nil, fmt.Errorf("authorize+MFA: %w", err)
	}

	session, err := authenticateApply(client, code, state)
	if err != nil {
		return nil, fmt.Errorf("/authenticate/apply: %w", err)
	}
	session.Email = email

	return session, nil
}

func headlessUUID4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func newHeadlessSurfClient() *surf.Client {
	return surf.NewClient().
		Builder().
		NotFollowRedirects().
		ForwardHeadersOnRedirect().
		Session().
		Impersonate().
		Chrome().
		Build().
		Unwrap()
}

var bffHeaderMap = map[string]string{
	"accept":           "application/json",
	"x-bff-key":        bffKey,
	"x-client-app-id":  "front-web",
	"x-client-channel": "web",
	"x-api-env":        "production",
	"x-market-locale":  "fr_FR",
	"x-app-version":    "v9.99.9",
	"content-type":     "application/json",
	"origin":           "https://www.sncf-connect.com",
	"referer":          "https://www.sncf-connect.com/",
}

func bffAuthenticate(client *surf.Client, state string) (string, error) {
	u := bffBase + "/api/v2/authenticate?" + url.Values{
		"redirectUri": {webRedirect},
		"state":       {state},
		"screenHint":  {"SIGN_IN"},
		"channel":     {"web"},
		"market":      {"fr_FR"},
	}.Encode()

	r := client.Get(g.String(u)).SetHeaders(bffHeaderMap).Do()
	if r.IsErr() {
		return "", r.Err()
	}
	resp := r.Unwrap()

	loc := string(resp.Headers.Get("location"))
	if loc == "" || !strings.Contains(loc, "authorize") {
		return "", fmt.Errorf("expected redirect to Auth0, got %d: %.200s",
			int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	return loc, nil
}

func coAuthenticate(client *surf.Client, email, password string) (string, error) {
	type coReq struct {
		ClientID       string `json:"client_id"`
		CredentialType string `json:"credential_type"`
		Username       string `json:"username"`
		Password       string `json:"password"`
	}
	body, err := json.Marshal(coReq{
		ClientID: publicClient, CredentialType: "password",
		Username: email, Password: password,
	})
	if err != nil {
		return "", err
	}

	r := client.Post(g.String(auth0Domain + "/co/authenticate")).
		SetHeaders(map[string]string{
			"content-type": "application/json",
			"origin":       auth0Domain,
		}).
		Body(string(body)).
		Do()
	if r.IsErr() {
		return "", r.Err()
	}
	resp := r.Unwrap()

	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return "", fmt.Errorf("credentials rejected (%d): %.200s",
			int(resp.StatusCode), resp.Body.Bytes().Ok())
	}
	var result struct {
		LoginTicket string `json:"login_ticket"`
	}
	if err := resp.Body.JSON(&result); err != nil || result.LoginTicket == "" {
		return "", fmt.Errorf("no login_ticket in response")
	}
	return result.LoginTicket, nil
}

func authorizeWithMFA(client *surf.Client, auth0URL, loginTicket string, otpFunc func(string) (string, error)) (string, error) {
	parsed, _ := url.Parse(auth0URL)
	q := parsed.Query()
	q.Set("login_ticket", loginTicket)
	q.Set("realm", "sncf")
	parsed.RawQuery = q.Encode()

	currentURL := parsed.String()
	var finalBody []byte
	for i := 0; i < 10; i++ {
		r := client.Get(g.String(currentURL)).Do()
		if r.IsErr() {
			return "", r.Err()
		}
		resp := r.Unwrap()

		if resp.StatusCode == 200 { //nolint:usestdlibvars
			finalBody = resp.Body.Bytes().Ok()
			break
		}
		loc := string(resp.Headers.Get("location"))
		if loc == "" {
			return "", fmt.Errorf("stuck at %d with no redirect", int(resp.StatusCode))
		}
		if strings.HasPrefix(loc, "/") {
			currentURL = auth0Domain + loc
		} else {
			currentURL = loc
		}
	}

	stateRe := regexp.MustCompile(`name="state"\s+value="([^"]+)"`)
	m := stateRe.FindSubmatch(finalBody)
	if m == nil {
		return "", fmt.Errorf("no MFA detect page found")
	}
	mfaState := string(m[1])

	formData := url.Values{
		"state": {mfaState}, "action": {"default"},
		"js-available": {"true"}, "webauthn-available": {"false"},
		"is-brave": {"false"}, "webauthn-platform-available": {"false"},
	}
	currentURL = auth0Domain + "/u/mfa-detect-browser-capabilities"
	bodyStr := formData.Encode()

	for i := 0; i < 5; i++ {
		var r g.Result[*surf.Response]
		if bodyStr != "" {
			r = client.Post(g.String(currentURL)).
				SetHeaders(map[string]string{
					"content-type": "application/x-www-form-urlencoded",
					"origin":       auth0Domain,
				}).
				Body(bodyStr).
				Do()
		} else {
			r = client.Get(g.String(currentURL)).Do()
		}
		if r.IsErr() {
			return "", r.Err()
		}
		resp := r.Unwrap()

		if resp.StatusCode == 200 { //nolint:usestdlibvars
			finalBody = resp.Body.Bytes().Ok()
			break
		}
		loc := string(resp.Headers.Get("location"))
		if loc == "" {
			break
		}
		if strings.HasPrefix(loc, "/") {
			loc = auth0Domain + loc
		}
		currentURL = loc
		bodyStr = ""
	}

	emailState := mfaState
	maskedEmail := "your email"
	atobRe := regexp.MustCompile(`atob\("([^"]+)"\)`)
	if am := atobRe.FindSubmatch(finalBody); am != nil {
		decoded, err := base64.StdEncoding.DecodeString(string(am[1]))
		if err == nil {
			var ctx struct {
				Transaction struct {
					State string `json:"state"`
				} `json:"transaction"`
				Screen struct {
					Data struct {
						Email string `json:"email"`
					} `json:"data"`
				} `json:"screen"`
			}
			if json.Unmarshal(decoded, &ctx) == nil {
				if ctx.Transaction.State != "" {
					emailState = ctx.Transaction.State
				}
				if ctx.Screen.Data.Email != "" {
					maskedEmail = ctx.Screen.Data.Email
				}
			}
		}
	}

	otp, err := otpFunc(maskedEmail)
	if err != nil {
		return "", fmt.Errorf("OTP input: %w", err)
	}

	otpForm := url.Values{
		"state": {emailState}, "action": {"default"}, "code": {otp},
	}
	currentURL = auth0Domain + "/u/mfa-email-challenge"

	for i := 0; i < 10; i++ {
		var r g.Result[*surf.Response]
		if i == 0 {
			r = client.Post(g.String(currentURL)).
				SetHeaders(map[string]string{
					"content-type": "application/x-www-form-urlencoded",
					"origin":       auth0Domain,
				}).
				Body(otpForm.Encode()).
				Do()
		} else {
			r = client.Get(g.String(currentURL)).Do()
		}
		if r.IsErr() {
			return "", r.Err()
		}
		resp := r.Unwrap()

		loc := string(resp.Headers.Get("location"))
		if loc == "" {
			return "", fmt.Errorf("OTP submission returned %d with no redirect (wrong code?)", int(resp.StatusCode))
		}

		if strings.Contains(loc, "code=") {
			parsed, _ := url.Parse(loc)
			code := parsed.Query().Get("code")
			if code != "" {
				return code, nil
			}
		}

		if strings.HasPrefix(loc, "ivts://") {
			parsed, _ := url.Parse(loc)
			code := parsed.Query().Get("code")
			if code != "" {
				return code, nil
			}
			return "", fmt.Errorf("ivts:// callback without code")
		}

		if strings.HasPrefix(loc, "/") {
			loc = auth0Domain + loc
		}
		currentURL = loc
	}

	return "", fmt.Errorf("could not obtain authorization code after OTP")
}

func authenticateApply(client *surf.Client, code, state string) (*Session, error) {
	fullRedirect := fmt.Sprintf("%s?code=%s&state=%s", webRedirect, url.QueryEscape(code), url.QueryEscape(state))
	type applyReq struct {
		Code        string `json:"code"`
		State       string `json:"state"`
		RedirectURI string `json:"redirectUri"`
	}
	body, err := json.Marshal(applyReq{Code: code, State: state, RedirectURI: fullRedirect})
	if err != nil {
		return nil, err
	}

	r := client.Post(g.String(bffBase + "/api/v2/authenticate/apply")).
		SetHeaders(bffHeaderMap).
		Body(string(body)).
		Do()
	if r.IsErr() {
		return nil, r.Err()
	}
	resp := r.Unwrap()

	if resp.StatusCode != 200 { //nolint:usestdlibvars
		return nil, fmt.Errorf("apply failed (%d): %.300s",
			int(resp.StatusCode), resp.Body.Bytes().Ok())
	}

	var result struct {
		AccessToken  string `json:"accessToken"`
		IDToken      string `json:"idToken"`
		RefreshToken string `json:"refreshToken"`
		ExpireIn     int    `json:"expireIn"`
	}
	if err := resp.Body.JSON(&result); err != nil {
		return nil, fmt.Errorf("parse apply response: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("no accessToken in response")
	}

	return &Session{
		AccessToken:  result.AccessToken,
		IDToken:      result.IDToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(result.ExpireIn) * time.Second),
	}, nil
}
