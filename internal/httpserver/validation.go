package httpserver

import "fmt"

// validate checks required HTTP dependencies.
func validate(dependencies Dependencies) error {
	if dependencies.Messages == nil {
		return fmt.Errorf("message store is required")
	}
	if dependencies.Chats == nil {
		return fmt.Errorf("chat store is required")
	}
	if dependencies.Enqueuer == nil {
		return fmt.Errorf("enqueuer is required")
	}
	if dependencies.Messenger == nil {
		return fmt.Errorf("messenger is required")
	}

	return nil
}

// maxBodyBytes returns the configured body size limit.
// For example, 0 falls back to 1 MiB.
func maxBodyBytes(dependencies serverDeps) int64 {
	if dependencies.maxBodyBytes > 0 {
		return dependencies.maxBodyBytes
	}

	return 1 << 20
}
