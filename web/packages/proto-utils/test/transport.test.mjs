import assert from "node:assert/strict";
import test from "node:test";

import {
  encodeMultiSegmentPath,
  encodePathSegment,
} from "@servora/proto-utils/transport";

test("encodePathSegment encodes reserved URI characters and RFC 3986 sub-delimiters", () => {
  assert.equal(encodePathSegment("u_123"), "u_123");
  assert.equal(encodePathSegment("acme team/alpha"), "acme%20team%2Falpha");
  assert.equal(encodePathSegment("!'()*"), "%21%27%28%29%2A");
  assert.equal(encodePathSegment(42), "42");
  assert.equal(encodePathSegment("管理员"), "%E7%AE%A1%E7%90%86%E5%91%98");
});

test("encodeMultiSegmentPath preserves slash separators while encoding each segment", () => {
  assert.equal(
    encodeMultiSegmentPath("tenants/acme team/users/u!(1)*"),
    "tenants/acme%20team/users/u%21%281%29%2A",
  );
  assert.equal(encodeMultiSegmentPath("users/u_001"), "users/u_001");
});
