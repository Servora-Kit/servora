import assert from "node:assert/strict";
import test from "node:test";

import {
  ApiError,
  isKratosReason,
  parseKratosError,
  parseKratosErrorBody,
} from "../dist/errors.js";

const validBody = {
  code: 404,
  reason: "USER_ERROR_REASON_NOT_FOUND",
  message: "user not found",
};


test("parseKratosErrorBody accepts the envelope, metadata, and extra fields", () => {
  assert.deepEqual(parseKratosErrorBody(validBody), validBody);

  const body = {
    ...validBody,
    metadata: { tenant: "demo", requestId: "request-1" },
    futureField: true,
  };
  assert.deepEqual(parseKratosErrorBody(body), body);
});

test("parseKratosErrorBody rejects invalid core fields without coercion", () => {
  const invalidBodies = [
    undefined,
    null,
    "error",
    [],
    { ...validBody, code: "404" },
    { ...validBody, code: 404.5 },
    { ...validBody, code: Number.NaN },
    { ...validBody, code: Number.POSITIVE_INFINITY },
    { ...validBody, reason: 404 },
    { ...validBody, message: null },
  ];

  for (const body of invalidBodies) {
    assert.equal(parseKratosErrorBody(body), null);
  }
});

test("parseKratosErrorBody rejects invalid metadata", () => {
  const invalidMetadata = [
    null,
    undefined,
    [],
    { tenant: 1 },
    { tenant: "demo", requestId: false },
  ];

  for (const metadata of invalidMetadata) {
    assert.equal(parseKratosErrorBody({ ...validBody, metadata }), null);
  }
});

test("parseKratosErrorBody returns null when unknown property access throws", () => {
  const body = new Proxy(
    {},
    {
      get() {
        throw new Error("unreadable");
      },
    },
  );

  assert.equal(parseKratosErrorBody(body), null);
});

test("parseKratosError only maps HTTP errors and matches reasons exactly", () => {
  const httpError = new ApiError({
    kind: "http",
    message: "request failed",
    httpStatus: 404,
    responseBody: validBody,
    service: "UserHTTPService",
    method: "GetUser",
  });
  const networkError = new ApiError({
    kind: "network",
    message: "network failed",
    responseBody: validBody,
    service: "UserHTTPService",
    method: "GetUser",
  });

  assert.deepEqual(parseKratosError(httpError), validBody);
  assert.equal(parseKratosError(networkError), null);
  assert.equal(isKratosReason(httpError, "USER_ERROR_REASON_NOT_FOUND"), true);
  assert.equal(isKratosReason(httpError, "user_error_reason_not_found"), false);
  assert.equal(isKratosReason(httpError, "NOT_FOUND"), false);
  assert.equal(isKratosReason(networkError, validBody.reason), false);
  for (const kind of ["timeout", "cancelled"]) {
    const error = new ApiError({
      kind,
      message: "请求未完成",
      responseBody: validBody,
      service: "UserHTTPService",
      method: "GetUser",
    });
    assert.equal(parseKratosError(error), null);
    assert.equal(isKratosReason(error, validBody.reason), false);
  }
});


test('removed subpaths are not exported', async () => {
  const removed = [
    '@servora/proto-utils/client',
    '@servora/proto-utils/client/request',
    '@servora/proto-utils/client/errors',
    '@servora/proto-utils/request',
    '@servora/proto-utils/crud',
    '@servora/proto-utils/error-reasons/servora/crud/v1/errors',
  ]

  for (const specifier of removed) {
    await assert.rejects(import(specifier), (error) => {
      assert.equal(error.code, 'ERR_PACKAGE_PATH_NOT_EXPORTED')
      return true
    })
  }
})
