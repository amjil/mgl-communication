package domain

import "fmt"

const (
	ErrCodeInvalidRequest       = "INVALID_REQUEST"
	ErrCodeUnauthorized         = "UNAUTHORIZED"
	ErrCodeForbidden            = "FORBIDDEN"
	ErrCodeDeviceNotFound       = "DEVICE_NOT_FOUND"
	ErrCodeInvalidToken         = "INVALID_TOKEN"
	ErrCodeProviderNotAvailable = "PROVIDER_NOT_AVAILABLE"
	ErrCodeProviderAuthError    = "PROVIDER_AUTH_ERROR"
	ErrCodeProviderRateLimit    = "PROVIDER_RATE_LIMIT"
	ErrCodeProviderTempError    = "PROVIDER_TEMPORARY_ERROR"
	ErrCodeMessageNotFound      = "MESSAGE_NOT_FOUND"
	ErrCodeInternalError        = "INTERNAL_ERROR"
)

type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewAppError(code, message string, httpStatus int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: httpStatus}
}

func InvalidRequest(msg string) *AppError {
	return NewAppError(ErrCodeInvalidRequest, msg, 400)
}

func Unauthorized(msg string) *AppError {
	return NewAppError(ErrCodeUnauthorized, msg, 401)
}

func Forbidden(msg string) *AppError {
	return NewAppError(ErrCodeForbidden, msg, 403)
}

func DeviceNotFound() *AppError {
	return NewAppError(ErrCodeDeviceNotFound, "Device not found", 404)
}

func MessageNotFound() *AppError {
	return NewAppError(ErrCodeMessageNotFound, "Message not found", 404)
}

func Internal(msg string) *AppError {
	return NewAppError(ErrCodeInternalError, msg, 500)
}
