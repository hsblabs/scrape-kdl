package executor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type URLPolicy func(context.Context, *url.URL) error

type urlPolicyError struct {
	url   string
	cause error
}

func (e *urlPolicyError) Error() string {
	return fmt.Sprintf("URL %q rejected by policy: %v", e.url, e.cause)
}
func (e *urlPolicyError) Unwrap() error { return e.cause }

func enforceURLPolicy(ctx context.Context, target string, policy URLPolicy) error {
	if policy == nil {
		return nil
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return &ExecutionError{Code: "E_URL_INVALID", Message: err.Error(), Cause: err}
	}
	if err := policy(ctx, parsed); err != nil {
		cause := &urlPolicyError{url: target, cause: err}
		return &ExecutionError{Code: "E_URL_POLICY", Message: cause.Error(), Cause: cause}
	}
	return nil
}

func clientWithURLPolicy(client *http.Client, policy URLPolicy) *http.Client {
	if policy == nil {
		return client
	}
	clone := *client
	previous := clone.CheckRedirect
	clone.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if err := policy(request.Context(), request.URL); err != nil {
			return &urlPolicyError{url: request.URL.String(), cause: err}
		}
		if previous != nil {
			return previous(request, via)
		}
		return nil
	}
	return &clone
}

func convertHTTPPolicyError(err error) error {
	var policy *urlPolicyError
	if errors.As(err, &policy) {
		return &ExecutionError{Code: "E_URL_POLICY", Message: policy.Error(), Cause: policy}
	}
	return nil
}
