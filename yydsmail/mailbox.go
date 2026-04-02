package yydsmail

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type CreateMailboxRequest struct {
	Address            string `json:"address,omitempty"`
	Domain             string `json:"domain,omitempty"`
	AutoDomainStrategy string `json:"autoDomainStrategy,omitempty"`
}

type RefreshTokenRequest struct {
	Address string `json:"address"`
}

type Mailbox struct {
	ID           string `json:"id"`
	Address      string `json:"address"`
	Token        string `json:"token,omitempty"`
	InboxType    string `json:"inboxType"`
	Source       string `json:"source"`
	ExpiresAt    string `json:"expiresAt"`
	IsActive     bool   `json:"isActive"`
	MessageCount int    `json:"messageCount,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

func (c *Client) CreateMailbox(ctx context.Context, reqBody CreateMailboxRequest) (*Mailbox, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/accounts", nil, reqBody)
	if err != nil {
		return nil, err
	}

	var resp apiResponse[Mailbox]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

func (c *Client) RefreshMailboxToken(ctx context.Context, address string) (*Mailbox, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/token", nil, RefreshTokenRequest{Address: address})
	if err != nil {
		return nil, err
	}

	var resp apiResponse[Mailbox]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

func (c *Client) GetCurrentMailbox(ctx context.Context) (*Mailbox, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/accounts/me", nil, nil)
	if err != nil {
		return nil, err
	}

	var resp apiResponse[Mailbox]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

func (c *Client) GetMailbox(ctx context.Context, id string) (*Mailbox, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/accounts/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return nil, err
	}

	var resp apiResponse[Mailbox]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

func (c *Client) DeleteMailbox(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/accounts/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return err
	}

	if err := c.do(req, nil); err != nil {
		return fmt.Errorf("delete mailbox %s: %w", id, err)
	}

	return nil
}
