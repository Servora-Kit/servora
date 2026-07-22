package crud

import (
	"encoding/base64"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	"google.golang.org/protobuf/proto"
)

// CurrentPageTokenVersion is the only payload version understood by this release.
const CurrentPageTokenVersion uint32 = 1

// PageTokenCodec is the replaceable opaque transport for the fixed PageTokenPayload.
type PageTokenCodec interface {
	Encode(*crudpb.PageTokenPayload) (string, error)
	Decode(string) (*crudpb.PageTokenPayload, error)
}

// UnsignedPageTokenCodec uses deterministic Proto binary and unpadded Base64URL.
// It intentionally provides no integrity or confidentiality.
type UnsignedPageTokenCodec struct{}

// NewUnsignedPageTokenCodec returns the framework default codec.
func NewUnsignedPageTokenCodec() UnsignedPageTokenCodec { return UnsignedPageTokenCodec{} }

// Encode serializes one supported payload deterministically.
func (UnsignedPageTokenCodec) Encode(payload *crudpb.PageTokenPayload) (string, error) {
	if payload == nil {
		return "", invalidPageToken("page_token", "payload is nil")
	}
	if payload.GetVersion() != CurrentPageTokenVersion {
		return "", invalidPageToken(
			"page_token",
			"unsupported payload version %d",
			payload.GetVersion(),
		)
	}
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(payload)
	if err != nil {
		return "", invalidPageToken("page_token", "marshal payload: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

// Decode parses one unpadded Base64URL token and rejects unsupported versions.
func (UnsignedPageTokenCodec) Decode(token string) (*crudpb.PageTokenPayload, error) {
	encoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, invalidPageToken("page_token", "decode Base64URL: %v", err)
	}
	payload := new(crudpb.PageTokenPayload)
	if err := proto.Unmarshal(encoded, payload); err != nil {
		return nil, invalidPageToken("page_token", "unmarshal payload: %v", err)
	}
	if payload.GetVersion() != CurrentPageTokenVersion {
		return nil, invalidPageToken(
			"page_token",
			"unsupported payload version %d",
			payload.GetVersion(),
		)
	}
	return payload, nil
}

var _ PageTokenCodec = UnsignedPageTokenCodec{}
