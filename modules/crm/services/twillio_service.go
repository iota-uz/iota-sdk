package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

// TwilioService handles Twilio-related operations
type TwilioService struct {
	client        *twilio.RestClient
	config        *Config
	messagesSvc   MessagesService
	uploadsClient UploadsClient
}

// Config holds the Twilio service configuration
type Config struct {
	AccountSID  string
	AuthToken   string
	PhoneNumber string
}

// NewTwilioService creates a new instance of TwilioService
func NewTwilioService(config *Config, messagesSvc MessagesService, uploadsClient UploadsClient) *TwilioService {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: config.AccountSID,
		Password: config.AuthToken,
	})

	return &TwilioService{
		client:        client,
		config:        config,
		messagesSvc:   messagesSvc,
		uploadsClient: uploadsClient,
	}
}

// SendMessageDTO represents the data needed to send a message
type SendMessageDTO struct {
	Message       string
	ToPhoneNumber string
	MediaURL      string
}

// SendMessage sends a message using Twilio
func (s *TwilioService) SendMessage(ctx context.Context, data SendMessageDTO) error {
	params := &twilioApi.CreateMessageParams{}
	params.SetBody(data.Message)
	params.SetFrom(s.config.PhoneNumber)
	params.SetTo(data.ToPhoneNumber)

	if data.MediaURL != "" {
		params.SetMediaUrl([]string{data.MediaURL})
	}

	_, err := s.client.Api.CreateMessage(params)
	return err
}

// DownloadMediaDTO represents the data needed to download media
type DownloadMediaDTO struct {
	URL      string
	MimeType string
	Filename string
}

// DownloadMediaResultDTO represents the result of a media download
type DownloadMediaResultDTO struct {
	Filename string
	MimeType string
	Path     string
}

// downloadMedia downloads media from Twilio
func (s *TwilioService) downloadMedia(ctx context.Context, media DownloadMediaDTO) (*DownloadMediaResultDTO, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", media.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(s.config.AccountSID, s.config.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download media: %w", err)
	}
	defer resp.Body.Close()

	uploadResult, err := s.uploadsClient.UploadFile(ctx, UploadsParams{
		BucketName: "temp",
		File:       resp.Body,
		ObjectName: media.Filename,
		MimeType:   media.MimeType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &DownloadMediaResultDTO{
		Filename: media.Filename,
		MimeType: media.MimeType,
		Path:     uploadResult.Path,
	}, nil
}

// InboundMessageDTO represents an inbound message from Twilio
type InboundMessageDTO struct {
	SmsStatus    string
	NumMedia     string
	Body         string
	From         string
	MediaURL     map[string]string
	MediaContent map[string]string
}

// HandleWebhookEvent processes incoming webhook events from Twilio
func (s *TwilioService) HandleWebhookEvent(ctx context.Context, data InboundMessageDTO) error {
	if data.SmsStatus != "received" {
		return nil
	}

	numMedia, err := strconv.Atoi(data.NumMedia)
	if err != nil {
		return fmt.Errorf("invalid NumMedia value: %w", err)
	}

	var downloadedMedia []DownloadMediaResultDTO

	for i := 0; i < numMedia; i++ {
		mediaURL := data.MediaURL[fmt.Sprintf("MediaUrl%d", i)]
		contentType := data.MediaContent[fmt.Sprintf("MediaContentType%d", i)]

		parsedURL, err := url.Parse(mediaURL)
		if err != nil {
			return fmt.Errorf("failed to parse media URL: %w", err)
		}

		mediaSid := path.Base(parsedURL.Path)
		extension := getExtensionFromMimeType(contentType)
		filename := fmt.Sprintf("%s.%s", mediaSid, extension)

		downloadResult, err := s.downloadMedia(ctx, DownloadMediaDTO{
			URL:      mediaURL,
			MimeType: contentType,
			Filename: filename,
		})
		if err != nil {
			return fmt.Errorf("failed to download media: %w", err)
		}

		downloadedMedia = append(downloadedMedia, *downloadResult)
	}

	mediaData := make([]MessageMedia, len(downloadedMedia))
	for i, media := range downloadedMedia {
		mediaData[i] = MessageMedia{
			MinioTempPath: media.Path,
			Filename:      media.Filename,
			MimeType:      media.MimeType,
		}
	}

	err = s.messagesSvc.CreateClientMessage(ctx, CreateMessageParams{
		Message:     data.Body,
		PhoneNumber: data.From,
		Media:       mediaData,
	})
	if err != nil {
		return fmt.Errorf("failed to create client message: %w", err)
	}

	return nil
}

// MessageMedia represents media attached to a message
type MessageMedia struct {
	MinioTempPath string
	Filename      string
	MimeType      string
}

// CreateMessageParams represents parameters for creating a client message
type CreateMessageParams struct {
	Message     string
	PhoneNumber string
	Media       []MessageMedia
}

// MessagesService defines the interface for message-related operations
type MessagesService interface {
	CreateClientMessage(ctx context.Context, params CreateMessageParams) error
}

// UploadsParams represents parameters for uploading a file
type UploadsParams struct {
	BucketName string
	File       io.Reader
	ObjectName string
	MimeType   string
}

// UploadResult represents the result of a file upload
type UploadResult struct {
	Path string
}

// UploadsClient defines the interface for upload-related operations
type UploadsClient interface {
	UploadFile(ctx context.Context, params UploadsParams) (*UploadResult, error)
}

// Helper function to get file extension from MIME type
func getExtensionFromMimeType(mimeType string) string {
	// This is a simplified version. You might want to use a more comprehensive MIME type mapping
	mimeMap := map[string]string{
		"image/jpeg": "jpg",
		"image/png":  "png",
		"image/gif":  "gif",
		// Add more mappings as needed
	}

	if ext, ok := mimeMap[strings.ToLower(mimeType)]; ok {
		return ext
	}
	return "bin"
}
