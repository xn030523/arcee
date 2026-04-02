package yydsmail

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
)

type EmailAddress struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type Attachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
	DownloadURL string `json:"downloadUrl"`
}

type Message struct {
	ID             string         `json:"id"`
	InboxID        string         `json:"inboxId"`
	InboxIDLegacy  string         `json:"inbox_id"`
	From           EmailAddress   `json:"from"`
	To             []EmailAddress `json:"to"`
	EmailAddress   string         `json:"email_address"`
	FromAddress    string         `json:"from_address"`
	Subject        string         `json:"subject"`
	Text           string         `json:"text,omitempty"`
	Content        string         `json:"content,omitempty"`
	HTML           []string       `json:"html,omitempty"`
	HTMLContent    string         `json:"html_content,omitempty"`
	Seen           bool           `json:"seen"`
	HasHTML        bool           `json:"has_html"`
	HasAttachments bool           `json:"hasAttachments"`
	Size           int            `json:"size"`
	CreatedAt      string         `json:"createdAt"`
	Attachments    []Attachment   `json:"attachments,omitempty"`
}

var verifyURLPattern = regexp.MustCompile(`https://api\.arcee\.ai/app/v1/verify-email/[A-Za-z0-9_-]+`)

func (m Message) PrimaryText() string {
	if m.Content != "" {
		return m.Content
	}
	return m.Text
}

func (m Message) PrimaryHTML() string {
	if m.HTMLContent != "" {
		return m.HTMLContent
	}
	if len(m.HTML) > 0 {
		return m.HTML[0]
	}
	return ""
}

func (m Message) RecipientAddress() string {
	if m.EmailAddress != "" {
		return m.EmailAddress
	}
	if len(m.To) > 0 {
		return m.To[0].Address
	}
	return ""
}

func (m Message) SenderAddress() string {
	if m.FromAddress != "" {
		return m.FromAddress
	}
	return m.From.Address
}

func (m Message) ExtractVerifyEmailLink() string {
	for _, candidate := range []string{m.PrimaryText(), m.PrimaryHTML()} {
		if candidate == "" {
			continue
		}
		if link := verifyURLPattern.FindString(candidate); link != "" {
			return link
		}
	}
	return ""
}

type MessageList struct {
	Messages    []Message `json:"messages"`
	Total       int       `json:"total"`
	UnreadCount int       `json:"unreadCount"`
}

type MessageQuery struct {
	Address string
	Limit   int
}

type MarkReadResult struct {
	Mailbox     string `json:"mailbox"`
	Updated     int    `json:"updated"`
	AlreadySeen int    `json:"alreadySeen"`
	Total       int    `json:"total"`
}

type MessageSeenUpdate struct {
	ID   string `json:"id"`
	Seen bool   `json:"seen"`
}

type MessageSource struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

func (c *Client) ListMessages(ctx context.Context, query MessageQuery) (*MessageList, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/messages", buildMessageQuery(query), nil)
	if err != nil {
		return nil, err
	}

	var resp apiResponse[MessageList]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

func (c *Client) GetMessage(ctx context.Context, messageID, address string) (*Message, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/messages/"+url.PathEscape(messageID), buildAddressQuery(address), nil)
	if err != nil {
		return nil, err
	}

	var resp apiResponse[Message]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

func (c *Client) GetMessageSource(ctx context.Context, messageID, address string) (*MessageSource, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/sources/"+url.PathEscape(messageID), buildAddressQuery(address), nil)
	if err != nil {
		return nil, err
	}

	var resp apiResponse[MessageSource]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

func (c *Client) MarkAllMessagesRead(ctx context.Context, address string) (*MarkReadResult, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/messages/mark-read", buildAddressQuery(address), nil)
	if err != nil {
		return nil, err
	}

	var resp apiResponse[MarkReadResult]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

func (c *Client) MarkMessageRead(ctx context.Context, messageID, address string) (*MessageSeenUpdate, error) {
	req, err := c.newRequest(ctx, http.MethodPatch, "/messages/"+url.PathEscape(messageID), buildAddressQuery(address), map[string]bool{"seen": true})
	if err != nil {
		return nil, err
	}

	var resp apiResponse[MessageSeenUpdate]
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}

	return &resp.Data, nil
}

func (c *Client) DeleteMessage(ctx context.Context, messageID, address string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/messages/"+url.PathEscape(messageID), buildAddressQuery(address), nil)
	if err != nil {
		return err
	}

	if err := c.do(req, nil); err != nil {
		return fmt.Errorf("delete message %s: %w", messageID, err)
	}

	return nil
}

func buildMessageQuery(query MessageQuery) url.Values {
	values := buildAddressQuery(query.Address)
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func buildAddressQuery(address string) url.Values {
	if address == "" {
		return nil
	}

	values := url.Values{}
	values.Set("address", address)
	return values
}
