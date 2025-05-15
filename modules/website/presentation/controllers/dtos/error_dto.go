package dtos

// APIErrorResponse represents standardized API error responses
type APIErrorResponse struct {
	Message string `json:"message"` // Human-readable error message
	Code    string `json:"code"`    // Machine-readable error code
}

// NewAPIError creates a new API error response
func NewAPIError(message, code string) APIErrorResponse {
	return APIErrorResponse{
		Message: message,
		Code:    code,
	}
}

// Common error codes
const (
	ErrorCodeInvalidPhoneFormat = "INVALID_PHONE_FORMAT"
	ErrorCodeUnknownCountryCode = "UNKNOWN_COUNTRY_CODE"
	ErrorCodeInvalidRequest     = "INVALID_REQUEST"
	ErrorCodeInternalServer     = "INTERNAL_SERVER_ERROR"
	ErrorCodeNotFound           = "NOT_FOUND"
	ErrorCodeThreadNotFound     = "THREAD_NOT_FOUND"
)
