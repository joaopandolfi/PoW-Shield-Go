package web

const (
	ErrorCodeInternal         = 10
	ErrorCodeBadRequest       = 11
	ErrorCodeForbidden        = 12
	ErrorCodeRateLimited      = 13
	ErrorCodeUnavailable      = 14
	ErrorCodeNotAcceptable    = 15
	ErrorMessageInternal      = "internal error"
	ErrorMessageBadRequest    = "bad request"
	ErrorMessageForbidden     = "forbidden"
	ErrorMessageRateLimited   = "rate limit exceeded"
	ErrorMessageUnavailable   = "service unavailable"
	ErrorMessageNotAcceptable = "not acceptable"
)

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func DefaultErrorForStatus(status int) ErrorResponse {
	switch status {
	case 400:
		return ErrorResponse{Code: ErrorCodeBadRequest, Message: ErrorMessageBadRequest}
	case 403:
		return ErrorResponse{Code: ErrorCodeForbidden, Message: ErrorMessageForbidden}
	case 406:
		return ErrorResponse{Code: ErrorCodeNotAcceptable, Message: ErrorMessageNotAcceptable}
	case 429:
		return ErrorResponse{Code: ErrorCodeRateLimited, Message: ErrorMessageRateLimited}
	case 503:
		return ErrorResponse{Code: ErrorCodeUnavailable, Message: ErrorMessageUnavailable}
	default:
		return ErrorResponse{Code: ErrorCodeInternal, Message: ErrorMessageInternal}
	}
}
