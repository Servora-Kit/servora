package openfga

import (
	"testing"
)

func TestParseTupleComponents_UserPrincipal(t *testing.T) {
	tu := Tuple{User: "user:abc-123", Relation: "viewer", Object: "project:proj-1"}
	user, objectType, objectID := parseTupleComponents(tu)
	if user != "abc-123" {
		t.Errorf("user = %q, want abc-123", user)
	}
	if objectType != "project" {
		t.Errorf("objectType = %q, want project", objectType)
	}
	if objectID != "proj-1" {
		t.Errorf("objectID = %q, want proj-1", objectID)
	}
}

func TestParseTupleComponents_ServicePrincipal(t *testing.T) {
	tu := Tuple{User: "service:gateway", Relation: "caller", Object: "endpoint:ep-1"}
	user, objectType, objectID := parseTupleComponents(tu)
	if user != "gateway" {
		t.Errorf("user = %q, want gateway", user)
	}
	if objectType != "endpoint" {
		t.Errorf("objectType = %q, want endpoint", objectType)
	}
	if objectID != "ep-1" {
		t.Errorf("objectID = %q, want ep-1", objectID)
	}
}

func TestParseTupleComponents_OrgMember(t *testing.T) {
	tu := Tuple{User: "organization:org-1#member", Relation: "viewer", Object: "project:p1"}
	user, _, _ := parseTupleComponents(tu)
	if user != "org-1#member" {
		t.Errorf("user = %q, want org-1#member", user)
	}
}

func TestParseTupleComponents_NoColon(t *testing.T) {
	tu := Tuple{User: "rawid", Relation: "viewer", Object: "project:p1"}
	user, _, _ := parseTupleComponents(tu)
	if user != "" {
		t.Errorf("user = %q, want empty", user)
	}
}

func TestAffectedRelations_EmptyMap(t *testing.T) {
	c := &Client{}
	rels := c.affectedRelations("admin", "project")
	if len(rels) != 1 || rels[0] != "admin" {
		t.Errorf("rels = %v, want [admin]", rels)
	}
}

func TestAffectedRelations_NilMap(t *testing.T) {
	c := &Client{computedRelations: nil}
	rels := c.affectedRelations("admin", "project")
	if len(rels) != 1 || rels[0] != "admin" {
		t.Errorf("rels = %v, want [admin]", rels)
	}
}

func TestAffectedRelations_CustomMap(t *testing.T) {
	c := &Client{
		computedRelations: map[string][]string{
			"project": {"can_view", "can_edit"},
		},
	}
	rels := c.affectedRelations("admin", "project")
	if len(rels) != 3 {
		t.Errorf("len(rels) = %d, want 3", len(rels))
	}
	expected := map[string]bool{"admin": true, "can_view": true, "can_edit": true}
	for _, r := range rels {
		if !expected[r] {
			t.Errorf("unexpected relation %q", r)
		}
	}
}

func TestAffectedRelations_UnmatchedType(t *testing.T) {
	c := &Client{
		computedRelations: map[string][]string{
			"project": {"can_view", "can_edit"},
		},
	}
	rels := c.affectedRelations("admin", "tenant")
	if len(rels) != 1 || rels[0] != "admin" {
		t.Errorf("rels = %v, want [admin]", rels)
	}
}

func TestCheckCacheKey(t *testing.T) {
	key := checkCacheKey("user:abc", "viewer", "project", "p1")
	expected := "authz:check:user:abc:viewer:project:p1"
	if key != expected {
		t.Errorf("key = %q, want %q", key, expected)
	}
}

func TestListCacheKey(t *testing.T) {
	key := listCacheKey("user:abc", "viewer", "project")
	expected := "authz:list:user:abc:viewer:project"
	if key != expected {
		t.Errorf("key = %q, want %q", key, expected)
	}
}

func TestBoolStr(t *testing.T) {
	if boolStr(true) != "1" {
		t.Error("boolStr(true) != 1")
	}
	if boolStr(false) != "0" {
		t.Error("boolStr(false) != 0")
	}
}
