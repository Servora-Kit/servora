import assert from "node:assert/strict";
import { createServer } from "node:http";
import test from "node:test";

import {
  ApiError,
  createRequestHandler,
} from "../dist/client/request.js";

async function startProtoJSONServer(t) {
  const server = createServer((request, response) => {
    response.setHeader("Content-Type", "application/protojson");
    if (request.url === "/v1/failure") {
      response.statusCode = 409;
      response.end(
        JSON.stringify({
          code: 409,
          reason: "USER_ERROR_REASON_ALREADY_EXISTS",
          message: "user already exists",
        }),
      );
      return;
    }
    response.end(
      JSON.stringify({
        name: "tenants/acme/users/ada",
        totalSize: "1",
      }),
    );
  });
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  t.after(
    () =>
      new Promise((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
      }),
  );
  const address = server.address();
  assert(address && typeof address === "object");
  return `http://127.0.0.1:${address.port}`;
}

test("request handler parses successful application/protojson responses", async (t) => {
  const baseUrl = await startProtoJSONServer(t);
  const request = createRequestHandler({ baseUrl });

  const result = await request(
    { path: "/v1/user", method: "GET", body: null },
    { service: "UserHTTPService", method: "GetUser" },
  );

  assert.deepEqual(result, {
    name: "tenants/acme/users/ada",
    totalSize: "1",
  });
});

test("request handler preserves parsed ProtoJSON error bodies", async (t) => {
  const baseUrl = await startProtoJSONServer(t);
  const request = createRequestHandler({ baseUrl });

  await assert.rejects(
    request(
      { path: "/v1/failure", method: "POST", body: "{}" },
      { service: "UserHTTPService", method: "CreateUser" },
    ),
    (error) => {
      assert(error instanceof ApiError);
      assert.equal(error.httpStatus, 409);
      assert.deepEqual(error.responseBody, {
        code: 409,
        reason: "USER_ERROR_REASON_ALREADY_EXISTS",
        message: "user already exists",
      });
      return true;
    },
  );
});
