package yydsmail

import (
	"context"
	"fmt"
	"net/http"
)

type MailboxDump struct {
	RequestedAddress string
	ResolvedAddress  string
	List             *MessageList
	Messages         []Message
}

func (c *Client) DumpMailbox(ctx context.Context, address string, limit int) (*MailboxDump, error) {
	list, mailboxAddress, err := c.ListMessagesForAddress(ctx, address, limit)
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(list.Messages))
	for _, summary := range list.Messages {
		msg := summary
		detail, detailErr := c.GetMessage(ctx, summary.ID, mailboxAddress)
		if detailErr == nil && detail != nil {
			msg = *detail
		}
		messages = append(messages, msg)
	}

	return &MailboxDump{
		RequestedAddress: address,
		ResolvedAddress:  mailboxAddress,
		List:             list,
		Messages:         messages,
	}, nil
}

func (c *Client) ListMessagesForAddress(ctx context.Context, address string, limit int) (*MessageList, string, error) {
	list, err := c.ListMessages(ctx, MessageQuery{
		Address: address,
		Limit:   limit,
	})
	if err == nil {
		return list, address, nil
	}

	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != http.StatusNotFound {
		return nil, "", err
	}

	current, currentErr := c.GetCurrentMailbox(ctx)
	if currentErr != nil {
		return nil, "", fmt.Errorf("list messages for %s: %w; get current mailbox: %v", address, err, currentErr)
	}

	list, err = c.ListMessages(ctx, MessageQuery{
		Address: current.Address,
		Limit:   limit,
	})
	if err != nil {
		return nil, "", fmt.Errorf("requested mailbox %s not found; fallback mailbox %s also failed: %w", address, current.Address, err)
	}

	return list, current.Address, nil
}
