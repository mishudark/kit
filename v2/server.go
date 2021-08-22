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
	var r interface{} = response

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
}

// ServeHTTP implements http.Handler.
func (s Server[I, O]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	for _, f := range s.before {
		ctx = f(ctx, r)
	}

	req, err := s.dec(ctx, r)
	if err != nil {
		s.errorEncoder(ctx, err, w)
		return
	}

	resp, err := s.h(ctx, req)
	if err != nil {
		s.errorEncoder(ctx, err, w)
		return
	}

	for _, f := range s.after {
		ctx = f(ctx, w)
	}

	err = s.enc(ctx, w, resp)
	if err != nil {
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
	}

	for _, option := range options {
		option(s)
	}

	return s
}
