package crud_test

import (
	"encoding/base64"
	"testing"

	"errors"
	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
	"github.com/Servora-Kit/servora/core/crud"
	"google.golang.org/protobuf/proto"
)

func TestUnsignedPageTokenCodecRoundTrip(t *testing.T) {
	t.Parallel()

	codec := crud.NewUnsignedPageTokenCodec()
	payload := &crudpb.PageTokenPayload{
		Version:            crud.CurrentPageTokenVersion,
		ContextFingerprint: make([]byte, 32),
		Cursor: []*crudpb.CursorValue{{
			Value: &crudpb.CursorValue_StringValue{StringValue: "alice"},
		}},
	}

	token, err := codec.Encode(payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(token); err != nil {
		t.Fatalf("token is not unpadded Base64URL: %v", err)
	}
	decoded, err := codec.Decode(token)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !proto.Equal(decoded, payload) {
		t.Fatalf("decoded payload = %v, want %v", decoded, payload)
	}
	second, err := codec.Encode(payload)
	if err != nil {
		t.Fatalf("second Encode: %v", err)
	}
	if second != token {
		t.Fatalf("deterministic token mismatch: %q != %q", second, token)
	}
}

func TestUnsignedPageTokenCodecRejectsMalformedToken(t *testing.T) {
	t.Parallel()

	codec := crud.NewUnsignedPageTokenCodec()
	for _, token := range []string{"%%%", "", "AA"} {
		t.Run(token, func(t *testing.T) {
			t.Parallel()
			if _, err := codec.Decode(token); !crudpb.IsCrudErrorReasonInvalidPageToken(err) {
				t.Fatalf("Decode(%q) error = %v, want INVALID_PAGE_TOKEN", token, err)
			}
		})
	}
}

func TestUnsignedPageTokenCodecRejectsUnknownVersion(t *testing.T) {
	t.Parallel()

	codec := crud.NewUnsignedPageTokenCodec()
	payload := &crudpb.PageTokenPayload{
		Version:            crud.CurrentPageTokenVersion + 1,
		ContextFingerprint: make([]byte, 32),
	}
	bytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	if _, err := codec.Decode(token); !crudpb.IsCrudErrorReasonInvalidPageToken(err) {
		t.Fatalf("Decode error = %v, want INVALID_PAGE_TOKEN", err)
	}
}

func TestUnsignedPageTokenCodecClonesPayload(t *testing.T) {
	t.Parallel()

	codec := crud.NewUnsignedPageTokenCodec()
	payload := &crudpb.PageTokenPayload{
		Version:            crud.CurrentPageTokenVersion,
		ContextFingerprint: make([]byte, 32),
	}
	token, err := codec.Encode(payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := codec.Decode(token)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	decoded.ContextFingerprint[0] = 1
	decodedAgain, err := codec.Decode(token)
	if err != nil {
		t.Fatalf("second Decode: %v", err)
	}
	if decodedAgain.ContextFingerprint[0] != 0 {
		t.Fatal("Decode reused mutable payload memory")
	}
}

func TestPrepareListDecodesTokenBeforeBizAndPreservesSkip(t *testing.T) {
	t.Parallel()

	codec := crud.NewUnsignedPageTokenCodec()
	payload := &crudpb.PageTokenPayload{
		Version:            crud.CurrentPageTokenVersion,
		ContextFingerprint: make([]byte, 32),
	}
	token, err := codec.Encode(payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	preparer, err := crud.NewListPreparer(crud.WithPageTokenCodec(codec))
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	query, err := preparer.PrepareList(plan, crud.ListInput{PageToken: token, Skip: 7})
	if err != nil {
		t.Fatalf("PrepareList: %v", err)
	}
	if got, want := query.Skip(), int64(7); got != want {
		t.Fatalf("Skip() = %d, want %d", got, want)
	}
	decoded := query.PageTokenPayload()
	if decoded == nil || !proto.Equal(decoded, payload) {
		t.Fatalf("PageTokenPayload() = %v, want %v", decoded, payload)
	}
	decoded.ContextFingerprint[0] = 1
	if query.PageTokenPayload().ContextFingerprint[0] != 0 {
		t.Fatal("PageTokenPayload exposed mutable query state")
	}
}

func TestPrepareListWrapsCustomCodecErrors(t *testing.T) {
	t.Parallel()

	preparer, err := crud.NewListPreparer(crud.WithPageTokenCodec(failingPageTokenCodec{}))
	if err != nil {
		t.Fatalf("NewListPreparer: %v", err)
	}
	descriptor := examplev1.File_servora_example_v1_example_proto.Messages().ByName("User")
	plan := crud.MustBuildResourcePlan[*examplev1.User](descriptor)
	if _, err := preparer.PrepareList(plan, crud.ListInput{PageToken: "opaque"}); !crudpb.IsCrudErrorReasonInvalidPageToken(err) {
		t.Fatalf("PrepareList error = %v, want INVALID_PAGE_TOKEN", err)
	}
}

func TestNewListPreparerRejectsNilPageTokenCodec(t *testing.T) {
	t.Parallel()

	if _, err := crud.NewListPreparer(crud.WithPageTokenCodec(nil)); err == nil {
		t.Fatal("NewListPreparer accepted nil PageTokenCodec")
	}
}

type failingPageTokenCodec struct{}

func (failingPageTokenCodec) Encode(*crudpb.PageTokenPayload) (string, error) {
	return "", errors.New("encode failure")
}

func (failingPageTokenCodec) Decode(string) (*crudpb.PageTokenPayload, error) {
	return nil, errors.New("decode failure")
}
