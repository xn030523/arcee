package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"arcee/arcee"
	"arcee/yydsmail"
)

const configPath = "config.json"

type Config struct {
	Address            string `json:"address"`
	Password           string `json:"password"`
	APIKey             string `json:"api_key"`
	Domain             string `json:"domain"`
	AutoDomainStrategy string `json:"auto_domain_strategy"`
	Limit              int    `json:"limit"`
	PollInterval       string `json:"poll_interval"`
	PollTimeout        string `json:"poll_timeout"`
	Confirm            *bool  `json:"confirm"`
	Login              *bool  `json:"login"`
	SkipSignup         *bool  `json:"skip_signup"`
	DumpMessages       *bool  `json:"dump_messages"`
	Debug              *bool  `json:"debug"`
}

func main() {
	configFile := flag.String("config", configPath, "path to config file")
	flag.Parse()

	cfg, err := loadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	httpClient := &http.Client{Timeout: 20 * time.Second}
	mailClient := yydsmail.NewClient(
		yydsmail.WithAPIKey(cfg.APIKey),
		yydsmail.WithHTTPClient(httpClient),
	)
	arceeClient := arcee.NewClient(arcee.WithHTTPClient(httpClient))
	ctx := context.Background()

	if cfg.DumpMessagesEnabled() {
		if cfg.Address == "" {
			log.Fatal("config.address is required when dump_messages is true")
		}
		if err := dumpMailbox(ctx, mailClient, cfg.Address, cfg.MessageLimit()); err != nil {
			log.Fatal(err)
		}
		return
	}

	identity := &arcee.GeneratedIdentity{
		Email:    cfg.Address,
		Password: cfg.Password,
	}
	if cfg.SkipSignupEnabled() {
		if identity.Email == "" {
			log.Fatal("config.address is required when skip_signup is true")
		}
	} else {
		signupResp, err := arcee.ProvisionAndSignupFlow(
			ctx,
			arceeClient,
			mailClient,
			cfg.Address,
			cfg.Domain,
			cfg.ResolvedAutoDomainStrategy(),
		)
		if err != nil {
			log.Fatal(err)
		}
		identity = signupResp.Identity
		fmt.Printf("signup email=%s\n", identity.Email)
		fmt.Printf("password=%s\n", identity.Password)
		if cfg.DebugEnabled() && len(signupResp.Response.Body) > 0 {
			fmt.Printf("signup_ok status=%d\n", signupResp.Response.StatusCode)
			fmt.Printf("identity first_name=%s last_name=%s email=%s password=%s\n", identity.FirstName, identity.LastName, identity.Email, identity.Password)
			fmt.Printf("signup_response=%s\n", arcee.CompactJSON(signupResp.Response.Body))
		}
	}

	_, link, err := arcee.WaitForVerifyLink(
		ctx,
		mailClient,
		identity.Email,
		cfg.MessageLimit(),
		cfg.ResolvedPollInterval(),
		cfg.ResolvedPollTimeout(),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("verify_link=%s\n", link)

	if cfg.ConfirmEnabled() {
		status, err := arcee.ConfirmLink(ctx, httpClient, link)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("verified status=%d\n", status)
		if cfg.DebugEnabled() {
			fmt.Printf("verify_status=%d\n", status)
		}
	}

	if cfg.LoginEnabled() {
		if identity.Password == "" {
			log.Fatal("config.password is required when login is true")
		}
		loginResp, err := arcee.LoginAfterVerification(ctx, arceeClient, identity)
		if err != nil {
			log.Fatal(err)
		}
		printLoginResult(loginResp)
		return
	}
}

func loadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("config.api_key is required")
	}

	return &cfg, nil
}

func (c *Config) MessageLimit() int {
	if c.Limit > 0 {
		return c.Limit
	}
	return 10
}

func (c *Config) ResolvedAutoDomainStrategy() string {
	if c.AutoDomainStrategy != "" {
		return c.AutoDomainStrategy
	}
	return "random"
}

func (c *Config) ResolvedPollInterval() time.Duration {
	return parseDurationOrDefault(c.PollInterval, 5*time.Second)
}

func (c *Config) ResolvedPollTimeout() time.Duration {
	return parseDurationOrDefault(c.PollTimeout, 2*time.Minute)
}

func (c *Config) ConfirmEnabled() bool {
	return boolOrDefault(c.Confirm, true)
}

func (c *Config) LoginEnabled() bool {
	return boolOrDefault(c.Login, true)
}

func (c *Config) SkipSignupEnabled() bool {
	return boolOrDefault(c.SkipSignup, false)
}

func (c *Config) DumpMessagesEnabled() bool {
	return boolOrDefault(c.DumpMessages, false)
}

func (c *Config) DebugEnabled() bool {
	return boolOrDefault(c.Debug, false)
}

func boolOrDefault(value *bool, fallback bool) bool {
	if value != nil {
		return *value
	}
	return fallback
}

func parseDurationOrDefault(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	if value, err := time.ParseDuration(raw); err == nil {
		return value
	}
	return fallback
}

func dumpMailbox(ctx context.Context, client *yydsmail.Client, address string, limit int) error {
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	dump, err := client.DumpMailbox(pollCtx, address, limit)
	if err != nil {
		return err
	}

	if dump.RequestedAddress != dump.ResolvedAddress {
		fmt.Printf("requested_mailbox=%s not_found_using_fallback=%s\n", dump.RequestedAddress, dump.ResolvedAddress)
	}
	fmt.Printf("mailbox=%s\n", dump.ResolvedAddress)
	fmt.Printf("messages=%d unread=%d total=%d\n", len(dump.Messages), dump.List.UnreadCount, dump.List.Total)
	for i, msg := range dump.Messages {
		fmt.Printf("--- message[%d] ---\n", i)
		printMessage(msg)
	}

	return nil
}

func printMessage(msg yydsmail.Message) {
	fmt.Printf("id=%s\n", msg.ID)
	fmt.Printf("subject=%q\n", msg.Subject)
	fmt.Printf("to=%s\n", msg.RecipientAddress())
	fmt.Printf("from=%s\n", msg.SenderAddress())
	fmt.Printf("seen=%t has_html=%t has_attachments=%t\n", msg.Seen, msg.HasHTML, msg.HasAttachments)
	if msg.CreatedAt != "" {
		fmt.Printf("created_at=%s\n", msg.CreatedAt)
	}
	if msg.PrimaryText() != "" {
		fmt.Printf("text:\n%s\n", msg.PrimaryText())
	}
	if msg.PrimaryHTML() != "" {
		fmt.Printf("html:\n%s\n", msg.PrimaryHTML())
	}
	if link := msg.ExtractVerifyEmailLink(); link != "" {
		fmt.Printf("verify_link=%s\n", link)
	}
}

func printLoginResult(loginResp *arcee.LoginResult) {
	if loginResp == nil || len(loginResp.Response.Body) == 0 {
		return
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(loginResp.Response.Body, &payload); err == nil && payload.AccessToken != "" {
		fmt.Printf("access_token=%s\n", payload.AccessToken)
	}
}
