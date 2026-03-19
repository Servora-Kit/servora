import { createRequestHandler } from '@servora/api-client/request'
import type { RequestHandlerOptions } from '@servora/api-client/request'

import {
  createApplicationServiceClient,
  createAuthnServiceClient,
  createUserServiceClient,
} from '@servora/api-client/iam/service/v1/index'
import type {
  ApplicationService,
  AuthnService,
  UserService,
} from '@servora/api-client/iam/service/v1/index'

export interface IamClients {
  authn: AuthnService
  user: UserService
  application: ApplicationService
}

export function createIamClients(
  options: RequestHandlerOptions = {},
): IamClients {
  const handler = createRequestHandler(options)

  return {
    authn: createAuthnServiceClient(handler),
    user: createUserServiceClient(handler),
    application: createApplicationServiceClient(handler),
  }
}

export type { RequestHandlerOptions } from '@servora/api-client/request'
export { ApiError } from '@servora/api-client/request'
export type { ApiErrorKind, TokenStore, RequestHandler } from '@servora/api-client/request'
