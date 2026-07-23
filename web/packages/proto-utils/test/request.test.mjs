import assert from "node:assert/strict";
import { createServer } from "node:http";
import test from "node:test";

import {
  ApiError,
  createRequestHandler,
} from "../dist/client/request.js";

async function startJSONServer(t) {
  const receivedRequests = [];
  const server = createServer(async (request, response) => {
    let body = "";
    for await (const chunk of request) {
      body += chunk;
    }
    receivedRequests.push({
      accept: request.headers.accept,
      body,
      contentType: request.headers["content-type"],
      method: request.method,
      url: request.url,
    });

    response.setHeader("Content-Type", "application/json");
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
        totalSize: "9223372036854775807",
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
  return {
    baseUrl: `http://127.0.0.1:${address.port}`,
    receivedRequests,
  };
}
test("request handler uses application/json for successful requests", async (t) => {
  const { baseUrl, receivedRequests } = await startJSONServer(t);
  const request = createRequestHandler({ baseUrl });

  const result = await request(
    { path: "/v1/user", method: "GET", body: null },
    { service: "UserHTTPService", method: "GetUser" },
  );

  assert.deepEqual(result, {
    name: "tenants/acme/users/ada",
    totalSize: "9223372036854775807",
  });
  assert.deepEqual(receivedRequests, [
    {
      accept: "application/json",
      body: "",
      contentType: undefined,
      method: "GET",
      url: "/v1/user",
    },
  ]);
});
test("request handler uses application/json for error requests", async (t) => {
  const { baseUrl, receivedRequests } = await startJSONServer(t);
  const request = createRequestHandler({ baseUrl });
  const body = JSON.stringify({ revision: "9223372036854775807" });

  await assert.rejects(
    request(
      { path: "/v1/failure", method: "POST", body },
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

  assert.deepEqual(receivedRequests, [
    {
      accept: "application/json",
      body,
      contentType: "application/json",
      method: "POST",
      url: "/v1/failure",
    },
  ]);
});

