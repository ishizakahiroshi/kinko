package unlock

import (
	"context"
	"errors"
)

func providerErrorForContext(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return NewProviderError(ErrorTimeout)
	}
	if errors.Is(err, context.Canceled) {
		return NewProviderError(ErrorCanceled)
	}
	return NewProviderError(ErrorUnavailable)
}

func zeroBytes(raw []byte) {
	for i := range raw {
		raw[i] = 0
	}
}
