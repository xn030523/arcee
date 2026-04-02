package arcee

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"arcee/yydsmail"
)

type SignupResult struct {
	Identity *GeneratedIdentity
	Response *SignupResponse
}

type LoginResult struct {
	Identity *GeneratedIdentity
	Response *LoginResponse
}

func ProvisionAndSignupExistingMailbox(ctx context.Context, client *Client, address string) (*SignupResult, error) {
	password, err := GeneratePassword(12)
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}

	identity := &GeneratedIdentity{
		FirstName: "Lucas",
		LastName:  "Walker",
		Email:     address,
		Password:  password,
	}

	resp, err := client.Signup(ctx, SignupRequest{
		Email:     identity.Email,
		Password:  identity.Password,
		FirstName: identity.FirstName,
		LastName:  identity.LastName,
		Preferences: SignupPreferences{
			Theme:            "dark",
			Language:         "en",
			LanguageDetected: "zh-CN",
			OAuthUser:        false,
		},
	})
	if err != nil {
		return nil, err
	}

	return &SignupResult{Identity: identity, Response: resp}, nil
}

func ProvisionAndSignupFlow(ctx context.Context, client *Client, mailClient *yydsmail.Client, address string, domain string, autoDomain string) (*SignupResult, error) {
	if address != "" {
		return ProvisionAndSignupExistingMailbox(ctx, client, address)
	}

	identity, resp, err := client.ProvisionAndSignup(ctx, mailClient, ProvisionRequest{
		Mailbox: yydsmail.CreateMailboxRequest{
			Domain:             domain,
			AutoDomainStrategy: autoDomain,
		},
	})
	if err != nil {
		return nil, err
	}

	return &SignupResult{Identity: identity, Response: resp}, nil
}

func WaitForVerifyLink(
	ctx context.Context,
	client *yydsmail.Client,
	address string,
	limit int,
	pollInterval time.Duration,
	pollTimeout time.Duration,
) (*yydsmail.Message, string, error) {
	deadline := time.Now().Add(pollTimeout)
	for {
		pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		list, err := client.ListMessages(pollCtx, yydsmail.MessageQuery{
			Address: address,
			Limit:   limit,
		})
		cancel()
		if err != nil {
			return nil, "", err
		}

		for _, msg := range list.Messages {
			if link := msg.ExtractVerifyEmailLink(); link != "" {
				return &msg, link, nil
			}
		}

		if time.Now().After(deadline) {
			return nil, "", fmt.Errorf("verification email not found within %s", pollTimeout)
		}

		time.Sleep(pollInterval)
	}
}

func ConfirmLink(ctx context.Context, httpClient *http.Client, link string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return 0, fmt.Errorf("create verify request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("confirm verify link: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func LoginAfterVerification(ctx context.Context, client *Client, identity *GeneratedIdentity) (*LoginResult, error) {
	if identity == nil || identity.Email == "" || identity.Password == "" {
		return nil, fmt.Errorf("email and password are required for login")
	}

	resp, err := client.Login(ctx, LoginRequest{
		Email:      identity.Email,
		Password:   identity.Password,
		RememberMe: false,
	})
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		Identity: identity,
		Response: resp,
	}, nil
}

func CompactJSON(raw []byte) string {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return string(raw)
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		return string(raw)
	}

	return string(buf)
}
