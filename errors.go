package rpc

import "fmt"

type customError struct {
	message string
}

func (err customError) Error() string {
	return err.message
}

type RpcMethodMalformedError struct{ customError }
type RpcServiceNotFoundError struct{ customError }
type RpcMethodNotFoundError struct{ customError }
type RpcHTTPMethodNotAllowedError struct{ customError }
type RpcHTTPUnsupportedMediaTypeError struct{ customError }
type RpcCodecRequestMethodError struct{ customError }
type RpcCodecReadRequestError struct{ customError }
type RpcCodecWriteResponseError struct{ customError }

func NewRpcMethodMalformedError(format string, args ...interface{}) RpcMethodMalformedError {
	return RpcMethodMalformedError{customError{message: fmt.Sprintf(format, args...)}}
}

func NewRpcServiceNotFoundError(format string, args ...interface{}) RpcServiceNotFoundError {
	return RpcServiceNotFoundError{customError{message: fmt.Sprintf(format, args...)}}
}

func NewRpcMethodNotFoundError(format string, args ...interface{}) RpcMethodNotFoundError {
	return RpcMethodNotFoundError{customError{message: fmt.Sprintf(format, args...)}}
}

func NewRpcHTTPMethodNotAllowedError(format string, args ...interface{}) RpcHTTPMethodNotAllowedError {
	return RpcHTTPMethodNotAllowedError{customError{message: fmt.Sprintf(format, args...)}}
}

func NewRpcHTTPUnsupportedMediaTypeError(format string, args ...interface{}) RpcHTTPUnsupportedMediaTypeError {
	return RpcHTTPUnsupportedMediaTypeError{customError{message: fmt.Sprintf(format, args...)}}
}

func NewRpcCodecRequestMethodError(format string, args ...interface{}) RpcCodecRequestMethodError {
	return RpcCodecRequestMethodError{customError{message: fmt.Sprintf(format, args...)}}
}

func NewRpcCodecReadRequestError(format string, args ...interface{}) RpcCodecReadRequestError {
	return RpcCodecReadRequestError{customError{message: fmt.Sprintf(format, args...)}}
}

func NewRpcCodecWriteResponseError(format string, args ...interface{}) RpcCodecWriteResponseError {
	return RpcCodecWriteResponseError{customError{message: fmt.Sprintf(format, args...)}}
}
