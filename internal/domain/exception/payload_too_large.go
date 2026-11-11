package exception

import "net/http"

const CodePayloadTooLarge = "PAYLOAD_TOO_LARGE"

func NewPayloadTooLarge(message string) AppError {
	return &appError{status: http.StatusRequestEntityTooLarge, code: CodePayloadTooLarge, message: message}
}
