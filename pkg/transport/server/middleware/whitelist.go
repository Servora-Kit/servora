// Package middleware provides HTTP/gRPC middleware implementations.
package middleware

import (
	"context"
	"strings"
	"sync"

	"github.com/go-kratos/kratos/v2/middleware/selector"
)

// MatcherMode controls matching strategy
type MatcherMode int

const (
	// Exact matches the full operation string or method-only fallback
	Exact MatcherMode = iota
	// Prefix matches any operation with given prefix
	Prefix
)

// WhiteList holds whitelist data and provides selector helpers.
// Operations in the whitelist will SKIP the auth middleware.
type WhiteList struct {
	mu    sync.RWMutex
	items map[string]struct{}
	mode  MatcherMode
}

// NewWhiteList creates a WhiteList with optional initial ops and mode.
func NewWhiteList(mode MatcherMode, ops ...string) *WhiteList {
	w := &WhiteList{
		items: make(map[string]struct{}, len(ops)),
		mode:  mode,
	}
	for _, o := range ops {
		if o == "" {
			continue
		}
		w.items[normalizeOp(o)] = struct{}{}
	}
	return w
}

// normalizeOp trims leading slash and returns normalized op
func normalizeOp(op string) string {
	return strings.TrimPrefix(op, "/")
}

// Add appends operations to the whitelist (thread-safe)
func (w *WhiteList) Add(ops ...string) *WhiteList {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, o := range ops {
		if o == "" {
			continue
		}
		w.items[normalizeOp(o)] = struct{}{}
	}
	return w
}

// Set replaces the whitelist with provided operations
func (w *WhiteList) Set(ops []string) *WhiteList {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = make(map[string]struct{}, len(ops))
	for _, o := range ops {
		if o == "" {
			continue
		}
		w.items[normalizeOp(o)] = struct{}{}
	}
	return w
}

// Clear empties the whitelist
func (w *WhiteList) Clear() *WhiteList {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = map[string]struct{}{}
	return w
}

// Snapshot returns a slice copy of whitelist entries
func (w *WhiteList) Snapshot() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	res := make([]string, 0, len(w.items))
	for k := range w.items {
		res = append(res, k)
	}
	return res
}

// Merge creates a new WhiteList containing items from both whitelists
func (w *WhiteList) Merge(other *WhiteList) *WhiteList {
	w.mu.RLock()
	other.mu.RLock()
	defer w.mu.RUnlock()
	defer other.mu.RUnlock()

	newItems := make(map[string]struct{}, len(w.items)+len(other.items))
	for k := range w.items {
		newItems[k] = struct{}{}
	}
	for k := range other.items {
		newItems[k] = struct{}{}
	}

	return &WhiteList{
		items: newItems,
		mode:  w.mode,
	}
}

// isWhitelistedLocked checks match under read lock
func (w *WhiteList) isWhitelistedLocked(op string) bool {
	switch w.mode {
	case Exact:
		if _, ok := w.items[op]; ok {
			return true
		}
		// fallback to method-only if op contains '/'
		if i := strings.LastIndex(op, "/"); i >= 0 && i+1 < len(op) {
			method := op[i+1:]
			if _, ok := w.items[method]; ok {
				return true
			}
		}
		return false
	case Prefix:
		for k := range w.items {
			if strings.HasPrefix(op, k) {
				return true
			}
		}
		return false
	default:
		_, ok := w.items[op]
		return ok
	}
}

// IsWhitelisted returns true if operation is whitelisted
func (w *WhiteList) IsWhitelisted(op string) bool {
	if op == "" {
		return false
	}
	n := normalizeOp(op)
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isWhitelistedLocked(n)
}

// MatchFunc returns a selector.MatchFunc that returns true to execute middleware.
// It will return false (skip middleware) when op is whitelisted.
func (w *WhiteList) MatchFunc() selector.MatchFunc {
	return func(ctx context.Context, operation string) bool {
		if operation == "" {
			return true
		}
		op := normalizeOp(operation)
		w.mu.RLock()
		defer w.mu.RUnlock()
		// skip middleware when whitelisted
		return !w.isWhitelistedLocked(op)
	}
}
