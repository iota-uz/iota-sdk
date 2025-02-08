package cpassproviders

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/twilio/twilio-go"
	"github.com/twilio/twilio-go/client"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

// Config holds the Twilio service configuration
type Config struct {
	AccountSID string
	AuthToken  string
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

type InboundTwilioMessageDTO struct {
	MessageSid          string `json:"MessageSid"`
	SmsSid              string `json:"SmsSid"`
	SmsMessageSid       string `json:"SmsMessageSid"`
	AccountSid          string `json:"AccountSid"`
	MessagingServiceSid string `json:"MessagingServiceSid"`
	From                string `json:"From"`
	To                  string `json:"To"`
	Body                string `json:"Body"`
	NumMedia            int    `json:"NumMedia,string"` // JSON encodes as string, so we use `,string`
	NumSegments         int    `json:"NumSegments,string"`

	// Media-related parameters
	MediaContentTypes map[string]string `json:"MediaContentTypes"`
	MediaUrls         map[string]string `json:"MediaUrls"`

	// Geographic data
	FromCity    string `json:"FromCity"`
	FromState   string `json:"FromState"`
	FromZip     string `json:"FromZip"`
	FromCountry string `json:"FromCountry"`
	ToCity      string `json:"ToCity"`
	ToState     string `json:"ToState"`
	ToZip       string `json:"ToZip"`
	ToCountry   string `json:"ToCountry"`
}

// NewTwilioProvider creates a new instance of TwilioProvider
func NewTwilioProvider(params twilio.ClientParams, webhookURL string) Provider {
	restClient := twilio.NewRestClientWithParams(params)
	return &TwilioProvider{
		webhookURL: webhookURL,
		client:     restClient,
		validator:  client.NewRequestValidator(params.Password),
	}
}

// TwilioProvider handles Twilio-related operations
type TwilioProvider struct {
	webhookURL string
	client     *twilio.RestClient
	validator  client.RequestValidator
}

// SendMessage sends a message using Twilio
func (s *TwilioProvider) SendMessage(ctx context.Context, data SendMessageDTO) error {
	params := &twilioApi.CreateMessageParams{}
	params.SetBody(data.Message)
	params.SetFrom(data.From)
	params.SetTo(data.To)

	if data.MediaURL != "" {
		params.SetMediaUrl([]string{data.MediaURL})
	}

	_, err := s.client.Api.CreateMessage(params)
	return err
}

func (s *TwilioProvider) WebhookHandler(eventBus eventbus.EventBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		signature := r.Header.Get("X-Twilio-Signature")
		if signature == "" {
			log.Printf("Missing X-Twilio-Signature header")
			http.Error(w, "missing signature header", http.StatusBadRequest)
		}
		// Parse form params if not already parsed
		if err := r.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v", err)
			http.Error(w, "failed to parse form", http.StatusBadRequest)
		}
		// Convert form values to params map
		params := make(map[string]string)
		for key, values := range r.PostForm {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}
		b, err := json.MarshalIndent(params, "", "  ")
		if err != nil {
			log.Printf("Error marshalling params: %v", err)
		}
		log.Printf("Received webhook: %s", string(b))
		isValid := s.validator.Validate(s.webhookURL, params, signature)
		if !isValid {
			log.Printf("Invalid signature")
			http.Error(w, "invalid signature", http.StatusUnauthorized)
		}

		eventBus.Publish(&ReceivedMessageEvent{
			From: params["From"],
			To:   params["To"],
			Body: params["Body"],
		})
		w.WriteHeader(http.StatusOK)
	}
}

// downloadMedia downloads media from Twilio
//func (s *TwilioProvider) downloadMedia(ctx context.Context, media DownloadMediaDTO) (*DownloadMediaResultDTO, error) {
//	req, err := http.NewRequestWithContext(ctx, "GET", media.URL, nil)
//	if err != nil {
//		return nil, fmt.Errorf("failed to create request: %w", err)
//	}
//
//	req.SetBasicAuth(s.config.AccountSID, s.config.AuthToken)
//
//	resp, err := http.DefaultClient.Do(req)
//	if err != nil {
//		return nil, fmt.Errorf("failed to download media: %w", err)
//	}
//	defer resp.Body.Close()
//
//	uploadResult, err := s.uploadsClient.UploadFile(ctx, UploadsParams{
//		BucketName: "temp",
//		File:       resp.Body,
//		ObjectName: media.Filename,
//		MimeType:   media.MimeType,
//	})
//	if err != nil {
//		return nil, fmt.Errorf("failed to upload file: %w", err)
//	}
//
//	return &DownloadMediaResultDTO{
//		Filename: media.Filename,
//		MimeType: media.MimeType,
//		Path:     uploadResult.Path,
//	}, nil
//}

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
