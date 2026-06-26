package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
)

type Keyspace struct {
	namespace string
	version   atomic.Uint64
}

func NewKeyspace(namespace string) *Keyspace {
	ks := &Keyspace{namespace: strings.TrimSpace(namespace)}
	ks.version.Store(1)
	return ks
}

func (k *Keyspace) Version() uint64 {
	if k == nil {
		return 0
	}
	return k.version.Load()
}

func (k *Keyspace) Invalidate(_ context.Context) uint64 {
	if k == nil {
		return 0
	}
	return k.version.Add(1)
}

func (k *Keyspace) Key(parts ...any) string {
	prefix := "cache"
	if k != nil && k.namespace != "" {
		prefix = k.namespace
	}
	version := uint64(0)
	if k != nil {
		version = k.Version()
	}
	return fmt.Sprintf("%s:v%d:%s", prefix, version, StableKey(parts...))
}

func StableKey(parts ...any) string {
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		encoded = append(encoded, stableEncode(part))
	}
	sum := sha256.Sum256([]byte(strings.Join(encoded, "|")))
	return hex.EncodeToString(sum[:])
}

func stableEncode(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case []string:
		cp := append([]string(nil), x...)
		sort.Strings(cp)
		data, _ := json.Marshal(cp)
		return string(data)
	default:
		data, _ := json.Marshal(x)
		return string(data)
	}
}
