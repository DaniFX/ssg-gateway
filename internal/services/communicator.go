package services

import (
	"context"
	"fmt"

	"github.com/ssg/ssg-gateway/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CommunicatorClient struct {
	client communicatorClient
	conn   *grpc.ClientConn
}

type communicatorClient interface {
	SendEmail(ctx context.Context, in *sendEmailRequest, opts ...grpc.CallOption) (*sendEmailResponse, error)
}

type sendEmailRequest struct {
	To      string
	Subject string
	Body    string
	Type    int32
}

type sendEmailResponse struct {
	Success   bool
	Message   string
	MessageId string
}

func NewCommunicatorClient(cfg *config.Config) (*CommunicatorClient, error) {
	conn, err := grpc.Dial(
		cfg.CommunicatorConfig.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to communicator service: %w", err)
	}

	return &CommunicatorClient{
		conn: conn,
	}, nil
}

func (c *CommunicatorClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *CommunicatorClient) SendEmail(ctx context.Context, to, subject, body string, isHTML bool) (string, error) {
	emailType := int32(1)
	if isHTML {
		emailType = 2
	}

	req := &sendEmailRequest{
		To:      to,
		Subject: subject,
		Body:    body,
		Type:    emailType,
	}

	var resp sendEmailResponse
	err := c.conn.Invoke(ctx, "/communicator.Communicator/SendEmail", req, &resp)
	if err != nil {
		return "", fmt.Errorf("failed to send email: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf(resp.Message)
	}

	return resp.MessageId, nil
}
