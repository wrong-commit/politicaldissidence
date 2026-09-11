package registrarcheck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const goDaddyAvailableURL = "https://api.godaddy.com/v1/domains/available"

func lookupGoDaddy(hostname string, client *http.Client, creds func() (key, secret string, ok bool)) (Info, error) {
	info := Info{
		Source:        SourceGoDaddy,
		Hostname:      hostname,
		Purchaseable:  TriWeird,
		WeirdResponse: TriYes,
	}
	if creds == nil {
		creds = defaultCreds
	}
	key, secret, ok := creds()
	if !ok {
		info.Message = "missing GoDaddy credentials"
		return info, fmt.Errorf("%s", info.Message)
	}
	if client == nil {
		client = defaultClient()
	}

	u, err := url.Parse(goDaddyAvailableURL)
	if err != nil {
		info.Message = err.Error()
		return info, err
	}
	q := u.Query()
	q.Set("domain", hostname)
	q.Set("checkType", "FULL")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		info.Message = err.Error()
		return info, err
	}
	req.Header.Set("Authorization", "sso-key "+key+":"+secret)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		info.Message = err.Error()
		return info, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		info.Message = err.Error()
		return info, err
	}

	classified, cerr := classifyGoDaddyBody(resp.StatusCode, body)
	classified.Source = SourceGoDaddy
	classified.Hostname = hostname
	if cerr != nil {
		return classified, cerr
	}
	return classified, nil
}

func classifyGoDaddyBody(statusCode int, body []byte) (Info, error) {
	info := Info{Purchaseable: TriWeird, WeirdResponse: TriYes}
	if statusCode < 200 || statusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", statusCode)
		} else if len(msg) > 200 {
			msg = msg[:200]
		}
		info.Message = msg
		return info, fmt.Errorf("HTTP %d", statusCode)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		info.Message = "empty body"
		return info, fmt.Errorf("empty body")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		info.Message = "bad JSON"
		return info, err
	}

	availRaw, hasAvail := raw["available"]
	_, hasError := raw["error"]
	if !hasAvail {
		info.Message = "missing available"
		return info, fmt.Errorf("missing available")
	}

	var availBool bool
	if err := json.Unmarshal(availRaw, &availBool); err != nil {
		// Non-boolean available → ambiguous
		info.Purchaseable = TriWeird
		info.WeirdResponse = TriWeird
		info.Message = "available not boolean"
		return info, nil
	}

	if hasError {
		// Boolean present but error object too → ambiguous
		info.Purchaseable = TriWeird
		info.WeirdResponse = TriWeird
		info.Message = "available with error object"
		return info, nil
	}

	info.WeirdResponse = TriNo
	info.Message = ""
	if availBool {
		info.Purchaseable = TriYes
	} else {
		info.Purchaseable = TriNo
	}
	return info, nil
}
