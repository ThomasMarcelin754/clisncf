package main

import (
	"fmt"
	"time"

	"github.com/thomasmarcelin/sncf-cli/internal/api"
	"github.com/thomasmarcelin/sncf-cli/internal/auth"
)

// authedClient loads the session, refreshes if expired, returns a ready API client.
func authedClient() (*api.ConnectClient, error) {
	sess, err := auth.LoadSession()
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	if sess == nil {
		return nil, fmt.Errorf("not logged in — run `sncfcli auth login` first")
	}

	client, err := api.NewTokenClient(sess.AccessToken, sess.IDToken)
	if err != nil {
		return nil, err
	}

	if sess.Expired() {
		if sess.RefreshToken == "" {
			return nil, fmt.Errorf("session expired, no refresh token — run `sncfcli auth login`")
		}
		at, it, rt, expireIn, err := client.Refresh(sess.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("refresh failed: %w — run `sncfcli auth login`", err)
		}
		sess.AccessToken = at
		sess.IDToken = it
		sess.RefreshToken = rt
		sess.ExpiresAt = time.Now().Add(time.Duration(expireIn) * time.Second)
		if err := sess.Save(); err != nil {
			return nil, fmt.Errorf("save session: %w", err)
		}
	}

	return client, nil
}
