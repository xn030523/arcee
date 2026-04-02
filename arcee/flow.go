package arcee

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	attempt := 0
	for {
		attempt++
		pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		list, err := client.ListMessages(pollCtx, yydsmail.MessageQuery{
			Address: address,
			Limit:   limit,
		})
		cancel()
		if err != nil {
			return nil, "", err
		}

		debugf("poll attempt=%d address=%s messages=%d unread=%d\n", attempt, address, len(list.Messages), list.UnreadCount)
		for _, summary := range list.Messages {
			debugf("poll message id=%s subject=%q created_at=%s\n", summary.ID, summary.Subject, summary.CreatedAt)

			msg := summary
			detailCtx, detailCancel := context.WithTimeout(ctx, 30*time.Second)
			detail, detailErr := client.GetMessage(detailCtx, summary.ID, address)
			detailCancel()
			if detailErr != nil {
				debugf("poll detail_error id=%s err=%v\n", summary.ID, detailErr)
			} else if detail != nil {
				msg = *detail
				debugf("poll detail_loaded id=%s text=%t html=%t\n", msg.ID, msg.PrimaryText() != "", msg.PrimaryHTML() != "")
			}

			if link := msg.ExtractVerifyEmailLink(); link != "" {
				debugf("poll verify_link_found id=%s\n", msg.ID)
				return &msg, link, nil
			}
		}

		if time.Now().After(deadline) {
			return nil, "", fmt.Errorf("verification email not found within %s", pollTimeout)
		}

		time.Sleep(pollInterval)
	}
}

func debugf(format string, args ...any) {
	if os.Getenv("ARCEE_DEBUG") != "1" {
		return
	}
	fmt.Printf(format, args...)
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
