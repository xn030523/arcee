package arcee

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"

	"arcee/yydsmail"
)

const defaultBaseURL = "https://api.arcee.ai/app/v1"

var (
	firstNames = []string{
		"Lucas", "Ethan", "Mason", "Logan", "Noah", "Liam", "Owen", "Henry",
		"Chloe", "Ava", "Mila", "Nora", "Luna", "Ella", "Grace", "Zoe",
	}
	lastNames = []string{
		"Walker", "Bennett", "Parker", "Turner", "Collins", "Reed", "Brooks", "Hayes",
		"Cooper", "Price", "Ward", "Bailey", "Long", "Powell", "Cook", "Gray",
	}
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Option func(*Client)

func WithBaseURL(rawURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(rawURL, "/")
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func NewClient(opts ...Option) *Client {
	client := &Client{
		baseURL:    defaultBaseURL,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

type SignupPreferences struct {
	Theme            string `json:"theme"`
	Language         string `json:"language"`
	LanguageDetected string `json:"language_detected"`
	OAuthUser        bool   `json:"oauth_user"`
}

type SignupRequest struct {
	Email       string            `json:"email"`
	Password    string            `json:"password"`
	FirstName   string            `json:"first_name"`
	LastName    string            `json:"last_name"`
	Preferences SignupPreferences `json:"preferences"`
}

type GeneratedIdentity struct {
	FirstName string            `json:"first_name"`
	LastName  string            `json:"last_name"`
	Email     string            `json:"email"`
	Password  string            `json:"password"`
	Mailbox   *yydsmail.Mailbox `json:"mailbox,omitempty"`
}

type ProvisionRequest struct {
	Mailbox yydsmail.CreateMailboxRequest
}

type SignupResponse struct {
	StatusCode int             `json:"status_code"`
	Body       json.RawMessage `json:"body"`
}

type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

type LoginResponse struct {
	StatusCode int             `json:"status_code"`
	Body       json.RawMessage `json:"body"`
}

func (c *Client) ProvisionIdentity(ctx context.Context, mailClient *yydsmail.Client, req ProvisionRequest) (*GeneratedIdentity, error) {
	if mailClient == nil {
		return nil, fmt.Errorf("mail client is required")
	}

	firstName, err := randomChoice(firstNames)
	if err != nil {
		return nil, fmt.Errorf("generate first name: %w", err)
	}

	lastName, err := randomChoice(lastNames)
	if err != nil {
		return nil, fmt.Errorf("generate last name: %w", err)
	}

	mailboxReq := req.Mailbox
	if mailboxReq.Address == "" {
		prefix, err := buildMailboxPrefix(firstName, lastName)
		if err != nil {
			return nil, fmt.Errorf("generate mailbox prefix: %w", err)
		}
		mailboxReq.Address = prefix
	}

	mailbox, err := mailClient.CreateMailbox(ctx, mailboxReq)
	if err != nil {
		return nil, fmt.Errorf("create yyds mailbox: %w", err)
	}

	password, err := GeneratePassword(12)
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}

	return &GeneratedIdentity{
		FirstName: firstName,
		LastName:  lastName,
		Email:     mailbox.Address,
		Password:  password,
		Mailbox:   mailbox,
	}, nil
}

func (c *Client) Signup(ctx context.Context, req SignupRequest) (*SignupResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal signup request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/signup", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create signup request: %w", err)
	}

	if err := setBrowserHeaders(httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send signup request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read signup response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("arcee signup failed: status=%d body=%s", resp.StatusCode, string(rawBody))
	}

	return &SignupResponse{
		StatusCode: resp.StatusCode,
		Body:       rawBody,
	}, nil
}

func (c *Client) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal login request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/login", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create login request: %w", err)
	}

	if err := setBrowserHeaders(httpReq); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send login request: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read login response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("arcee login failed: status=%d body=%s", resp.StatusCode, string(rawBody))
	}

	return &LoginResponse{
		StatusCode: resp.StatusCode,
		Body:       rawBody,
	}, nil
}

func (c *Client) ProvisionAndSignup(ctx context.Context, mailClient *yydsmail.Client, req ProvisionRequest) (*GeneratedIdentity, *SignupResponse, error) {
	identity, err := c.ProvisionIdentity(ctx, mailClient, req)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.Signup(ctx, SignupRequest{
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
		return identity, nil, err
	}

	return identity, resp, nil
}

func GeneratePassword(length int) (string, error) {
	if length < 4 {
		return "", fmt.Errorf("password length must be at least 4")
	}

	lower := "abcdefghijklmnopqrstuvwxyz"
	upper := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits := "0123456789"
	special := "@#$%&*!?"
	allChars := lower + upper + digits + special

	lowerChar, err := randomChar(lower)
	if err != nil {
		return "", err
	}
	upperChar, err := randomChar(upper)
	if err != nil {
		return "", err
	}
	digitChar, err := randomChar(digits)
	if err != nil {
		return "", err
	}
	specialChar, err := randomChar(special)
	if err != nil {
		return "", err
	}

	parts := []byte{lowerChar, upperChar, digitChar, specialChar}

	for len(parts) < length {
		ch, err := randomChar(allChars)
		if err != nil {
			return "", err
		}
		parts = append(parts, ch)
	}

	if err := shuffle(parts); err != nil {
		return "", err
	}

	return string(parts), nil
}

func buildMailboxPrefix(firstName, lastName string) (string, error) {
	suffix, err := randomString("abcdefghijklmnopqrstuvwxyz0123456789", 6)
	if err != nil {
		return "", err
	}

	return strings.ToLower(firstName + lastName + suffix), nil
}

func randomChoice(values []string) (string, error) {
	if len(values) == 0 {
		return "", fmt.Errorf("empty choice set")
	}

	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(values))))
	if err != nil {
		return "", err
	}

	return values[index.Int64()], nil
}

func randomString(charset string, length int) (string, error) {
	buf := make([]byte, length)
	for i := range buf {
		ch, err := randomChar(charset)
		if err != nil {
			return "", err
		}
		buf[i] = ch
	}
	return string(buf), nil
}

func randomChar(charset string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
	if err != nil {
		return 0, err
	}
	return charset[n.Int64()], nil
}

func shuffle(values []byte) error {
	for i := len(values) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		j := int(n.Int64())
		values[i], values[j] = values[j], values[i]
	}
	return nil
}

func randomUUID() (string, error) {
	var raw [16]byte
	if _, err := io.ReadFull(rand.Reader, raw[:]); err != nil {
		return "", err
	}

	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	buf := make([]byte, 36)
	hex.Encode(buf[0:8], raw[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], raw[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], raw[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], raw[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], raw[10:16])

	return string(buf), nil
}

func setBrowserHeaders(httpReq *http.Request) error {
	requestID, err := randomUUID()
	if err != nil {
		return fmt.Errorf("generate x-request-id: %w", err)
	}
	sessionID, err := randomUUID()
	if err != nil {
		return fmt.Errorf("generate x-session-id: %w", err)
	}

	httpReq.Header.Set("Accept", "*/*")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Origin", "https://chat.arcee.ai")
	httpReq.Header.Set("Referer", "https://chat.arcee.ai/")
	httpReq.Header.Set("X-Request-Id", requestID)
	httpReq.Header.Set("X-Session-Id", sessionID)
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 Edg/146.0.0.0")

	return nil
}
