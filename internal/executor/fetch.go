package executor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/dom"
)

func fetchDocument(ctx context.Context, targetURL string, options Options) (*dom.Node, error) {
	if err := ctx.Err(); err != nil {
		return nil, &ExecutionError{Code: operationErrorCode("E_HTTP_FETCH", err), Message: err.Error(), Cause: err}
	}
	requestContext, cancel := context.WithTimeout(ctx, options.RequestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, &ExecutionError{Code: "E_HTTP_REQUEST", Message: err.Error(), Cause: err}
	}
	if options.Session != nil {
		for name, values := range options.Session.Headers {
			for _, value := range values {
				request.Header.Add(name, value)
			}
		}
		for _, cookie := range options.Session.Cookies {
			request.AddCookie(cookie)
		}
	}
	if request.Header.Get("User-Agent") == "" {
		request.Header.Set("User-Agent", options.UserAgent)
	}
	response, err := clientWithURLPolicy(options.HTTPClient, options.URLPolicy).Do(request)
	if err != nil {
		if policyErr := convertHTTPPolicyError(err); policyErr != nil {
			return nil, policyErr
		}
		return nil, &ExecutionError{Code: operationErrorCode("E_HTTP_FETCH", err), Message: err.Error(), Cause: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &ExecutionError{Code: "E_HTTP_STATUS", Message: fmt.Sprintf("unexpected HTTP status %s", response.Status)}
	}
	limited := io.LimitReader(response.Body, options.MaxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, &ExecutionError{Code: "E_HTTP_READ", Message: err.Error(), Cause: err}
	}
	if int64(len(body)) > options.MaxResponseBytes {
		return nil, &ExecutionError{Code: "E_HTTP_BODY_TOO_LARGE", Message: fmt.Sprintf("response exceeds %d bytes", options.MaxResponseBytes)}
	}
	decoded, err := decodeHTMLWithFallback(body, response.Header.Get("Content-Type"), options.CharsetDecoder)
	if err != nil {
		return nil, err
	}
	document, err := dom.ParseHTML(strings.NewReader(decoded))
	if err != nil {
		return nil, &ExecutionError{Code: "E_HTML_PARSE", Message: err.Error(), Cause: err}
	}
	return document, nil
}
