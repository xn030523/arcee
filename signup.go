package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"arcee/arcee"
	appconfig "arcee/config"
	"arcee/yydsmail"
)

func runSignup(cfg *appconfig.Config) {
	if cfg.Signup.APIKey == "" {
		log.Fatal("config.signup.api_key is required")
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}
	mailClient := yydsmail.NewClient(
		yydsmail.WithAPIKey(cfg.Signup.APIKey),
		yydsmail.WithHTTPClient(httpClient),
	)
	arceeClient := arcee.NewClient(arcee.WithHTTPClient(httpClient))
	ctx := context.Background()

	signupResp, err := arcee.ProvisionAndSignupFlow(
		ctx,
		arceeClient,
		mailClient,
		"",
		cfg.Signup.Domain,
		"random",
	)
	if err != nil {
		log.Fatal(err)
	}

	identity := signupResp.Identity
	fmt.Printf("signup email=%s\n", identity.Email)
	fmt.Printf("password=%s\n", identity.Password)

	_, link, err := arcee.WaitForVerifyLink(
		ctx,
		mailClient,
		identity.Email,
		10,
		2*time.Second,
		20*time.Second,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("verify_link=%s\n", link)

	status, err := arcee.ConfirmLink(ctx, httpClient, link)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("verified status=%d\n", status)

	loginResp, err := arcee.LoginAfterVerification(ctx, arceeClient, identity)
	if err != nil {
		log.Fatal(err)
	}

	accessToken := extractAccessToken(loginResp)
	if accessToken != "" {
		if err := appconfig.SaveAccessTokenFile(appconfig.DefaultAccessTokenPath, accessToken, identity.Email, identity.Password, link); err != nil {
			log.Fatal(err)
		}
	}

	printLoginResult(accessToken)
}

func extractAccessToken(loginResp *arcee.LoginResult) string {
	if loginResp == nil || len(loginResp.Response.Body) == 0 {
		return ""
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(loginResp.Response.Body, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.AccessToken)
}

func printLoginResult(accessToken string) {
	if accessToken != "" {
		fmt.Printf("access_token=%s\n", accessToken)
	}
}
