package kit

import (
	"context"
	"io"
	"net/http"

	"github.com/mishudark/errors"
	"github.com/mishudark/magic"
)

// DecodeSaveRequest decodes a POST, PUT or PATCH request
func DecodeRequest(ctx context.Context, r *http.Request, container interface{}) (err error) {
	switch r.Method {
	case "POST", "PUT", "PATCH":
		err = magic.Decode(r, &container,
			magic.JSON,
			magic.ChiRouter,
		)
	case "GET", "DELETE":
		err = magic.Decode(r, &container,
			magic.QueryParams,
			magic.ChiRouter,
		)
	default:
		err = errors.Errorf("method %s not supported", r.Method)
	}

	if err == io.EOF {
		err = errors.New("empty body")
	}

	return errors.E(err, "can not unmarshal request", errors.Unmarshal)
}
