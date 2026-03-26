package runtime

import "errors"

var (
	ErrPluginNil               = errors.New("plugin is nil")
	ErrPluginTypeEmpty         = errors.New("plugin type is empty")
	ErrPluginAlreadyRegistered = errors.New("plugin already registered")
	ErrPluginNotFound          = errors.New("plugin not found")
)
