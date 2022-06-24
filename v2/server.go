package kit

import (
	"context"
	"encoding/json"
	"net/http"
)

// RequestFunc may take information from an HTTP request and put it into a
// request context. In Servers, RequestFuncs are executed prior to invoking the
// endpoint.
type RequestFunc func(context.Context, *http.Request) context.Context

// ErrorEncoder is responsible for encoding an error to the ResponseWriter.
// Users are encouraged to use custom ErrorEncoders to encode HTTP errors to
// their clients, and will likely want to pass and check for their own error
// types. See the example shipping/handling service.
type ErrorEncoder func(ctx context.Context, err error, w http.ResponseWriter)

// NopRequestDecoder is a DecodeRequestFunc that can be used for requests that do not
// need to be decoded, and simply returns nil, nil.
func NopRequestDecoder(ctx context.Context, r *http.Request) (any, error) {
	return nil, nil
}

// ServerFinalizerFunc can be used to perform work at the end of an HTTP
// request, after the response has been written to the client. The principal
// intended use is for request logging. In addition to the response code
// provided in the function signature, additional response parameters are
// provided in the context under keys with the ContextKeyResponse prefix.
type ServerFinalizerFunc func(ctx context.Context, code int, r *http.Request)

// DecodeRequestFunc extracts a user-domain request object from an HTTP
// request object. It's designed to be used in HTTP servers, for server-side
// endpoints. One straightforward DecodeRequestFunc could be something that
// JSON decodes from the request body to the concrete request type.
type DecodeRequestFunc[I any] func(context.Context, *http.Request) (req I, err error)

// EncodeResponseFunc encodes the passed response object to the HTTP response
// writer. It's designed to be used in HTTP servers, for server-side
// endpoints. One straightforward EncodeResponseFunc could be something that
// JSON encodes the object directly to the response body.
type EncodeReponseFunc[O any] func(context.Context, http.ResponseWriter, O) error

// HandlerFunc performs the business logic for the service.
type HandlerFunc[I, O any] func(context.Context, I) (resp O, err error)

// DefaultErrorEncoder writes the error to the ResponseWriter, by default a
// content type of text/plain, a body of the plain text of the error, and a
// status code of 500. If the error implements Headerer, the provided headers
// will be applied to the response. If the error implements json.Marshaler, and
// the marshaling succeeds, a content type of application/json and the JSON
// encoded form of the error will be used. If the error implements StatusCoder,
// the provided StatusCode will be used instead of 500.
func DefaultErrorEncoder(_ context.Context, err error, w http.ResponseWriter) {
	contentType, body := "text/plain; charset=utf-8", []byte(err.Error())
	if marshaler, ok := err.(json.Marshaler); ok {
		if jsonBody, marshalErr := marshaler.MarshalJSON(); marshalErr == nil {
			contentType, body = "application/json; charset=utf-8", jsonBody
		}
	}
	w.Header().Set("Content-Type", contentType)
	if headerer, ok := err.(Headerer); ok {
		for k, values := range headerer.Headers() {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
	}
	code := http.StatusInternalServerError
	if sc, ok := err.(StatusCoder); ok {
		code = sc.StatusCode()
	}
	w.WriteHeader(code)
	w.Write(body)
}

// StatusCoder is checked by DefaultErrorEncoder. If an error value implements
// StatusCoder, the StatusCode will be used when encoding the error. By default,
// StatusInternalServerError (500) is used.
type StatusCoder interface {
	StatusCode() int
}

// Headerer is checked by DefaultErrorEncoder. If an error value implements
// Headerer, the provided headers will be applied to the response writer, after
// the Content-Type is set.
type Headerer interface {
	Headers() http.Header
}

// EncodeJSONResponse is a EncodeResponseFunc that serializes the response as a
// JSON object to the ResponseWriter. Many JSON-over-HTTP services can use it as
// a sensible default. If the response implements Headerer, the provided headers
// will be applied to the response. If the response implements StatusCoder, the
// provided StatusCode will be used instead of 200.
func EncodeJSONResponse[O any](_ context.Context, w http.ResponseWriter, response O) error {
	// convert response to an interface to check if it implements StatusCoder or Headerer
	var r any = response

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if headerer, ok := r.(Headerer); ok {
		for k, values := range headerer.Headers() {
			for _, v := range values {
				w.Header().Add(k, v)
			}
		}
	}
	code := http.StatusOK
	if sc, ok := r.(StatusCoder); ok {
		code = sc.StatusCode()
	}
	w.WriteHeader(code)
	if code == http.StatusNoContent {
		return nil
	}
	return json.NewEncoder(w).Encode(response)
}

// Server wraps an business logic service and implements http.Handler.
type Server[I, O any] struct {
	h            HandlerFunc[I, O]
	dec          DecodeRequestFunc[I]
	enc          EncodeReponseFunc[O]
	before       []RequestFunc
	after        []ServerResponseFunc
	errorEncoder ErrorEncoder
	finalizer    []ServerFinalizerFunc
	errorHandler ErrorHandler
}

// ServerErrorHandler is used to handle non-terminal errors. By default, non-terminal errors
// are ignored. This is intended as a diagnostic measure. Finer-grained control
// of error handling, including logging in more detail, should be performed in a
// custom ServerErrorEncoder or ServerFinalizer, both of which have access to
// the context.
func ServerErrorHandler[I, O any](errorHandler ErrorHandler) ServerOption[I, O] {
	return func(s *Server[I, O]) { s.errorHandler = errorHandler }
}

// ServerFinalizer is executed at the end of every HTTP request.
// By default, no finalizer is registered.
func ServerFinalizer[I, O any](f ...ServerFinalizerFunc) ServerOption[I, O] {
	return func(s *Server[I, O]) { s.finalizer = append(s.finalizer, f...) }
}

// ServeHTTP implements http.Handler.
func (s Server[I, O]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if len(s.finalizer) > 0 {
		iw := &interceptingWriter{w, http.StatusOK, 0}
		defer func() {
			ctx = context.WithValue(ctx, ContextKeyResponseHeaders, iw.Header())
			ctx = context.WithValue(ctx, ContextKeyResponseSize, iw.written)
			for _, f := range s.finalizer {
				f(ctx, iw.code, r)
			}
		}()
		w = iw
	}

	for _, f := range s.before {
		ctx = f(ctx, r)
	}

	req, err := s.dec(ctx, r)
	if err != nil {
		s.errorHandler.Handle(ctx, err)
		s.errorEncoder(ctx, err, w)
		return
	}

	resp, err := s.h(ctx, req)
	if err != nil {
		s.errorHandler.Handle(ctx, err)
		s.errorEncoder(ctx, err, w)
		return
	}

	for _, f := range s.after {
		ctx = f(ctx, w)
	}

	err = s.enc(ctx, w, resp)
	if err != nil {
		s.errorHandler.Handle(ctx, err)
		s.errorEncoder(ctx, err, w)
	}
}

// ServerOption sets an optional parameter for servers.
type ServerOption[I, O any] func(*Server[I, O])

// ServerBefore functions are executed on the HTTP request object before the
// request is decoded.
func ServerBefore[I, O any](before ...RequestFunc) ServerOption[I, O] {
	return func(s *Server[I, O]) {
		s.before = append(s.before, before...)
	}
}

// ServerAfter functions are executed on the HTTP response writer after the
// endpoint is invoked, but before anything is written to the client.
func ServerAfter[I, O any](after ...ServerResponseFunc) ServerOption[I, O] {
	return func(s *Server[I, O]) {
		s.after = append(s.after, after...)
	}
}

// ServerResponseFunc may take information from a request context and use it to
// manipulate a ResponseWriter. ServerResponseFuncs are only executed in
// servers, after invoking the endpoint but prior to writing a response.
type ServerResponseFunc func(context.Context, http.ResponseWriter) context.Context

// NewServer constructs a new server, which implements http.Handler.
func NewServer[I, O any](
	h HandlerFunc[I, O],
	dec DecodeRequestFunc[I],
	enc EncodeReponseFunc[O],
	options ...ServerOption[I, O],
) *Server[I, O] {
	s := &Server[I, O]{
		h:            h,
		dec:          dec,
		enc:          enc,
		errorEncoder: DefaultErrorEncoder,
		errorHandler: ErrorHandlerFunc(LogErrorHandler),
	}

	for _, option := range options {
		option(s)
	}

	return s
}

type interceptingWriter struct {
	http.ResponseWriter
	code    int
	written int64
}

// WriteHeader may not be explicitly called, so care must be taken to
// initialize w.code to its default value of http.StatusOK.
func (w *interceptingWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *interceptingWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.written += int64(n)
	return n, err
}

var _ StatusCoder = (*ReponseCode)(nil)
var _ json.Marshaler = (*ReponseCode)(nil)

// ReponseCode implements StatusCoder and json.Marshaler
type ReponseCode struct {
	resp any
	code int
}

// StatusCode returns the given status code
func (r *ReponseCode) StatusCode() int {
	return r.code
}

// StatusCode returns the marshaling result of provided response
func (r *ReponseCode) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.resp)
}

// AddStatusCode add the given code to the encoder using the provieded response
func AddStatusCode(resp any, code int) *ReponseCode {
	return &ReponseCode{
		resp: resp,
		code: code,
	}
}
