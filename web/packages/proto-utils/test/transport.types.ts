import {
  encodeMultiSegmentPath,
  encodePathSegment,
  type ClientTransport,
  type DuplexStream,
  type ServerStream,
  type TransportMeta,
} from "@servora/proto-utils/transport";

const meta: TransportMeta = { service: "UserService", method: "GetUser" };

const transport: ClientTransport = {
  async unary<T>(): Promise<T> {
    throw new Error("not called");
  },
  serverStream<T>(): ServerStream<T> {
    return {
      onEvent: () => () => {},
      onError: () => {},
      close: () => {},
    };
  },
  duplexStream<TIn, TOut>(): DuplexStream<TIn, TOut> {
    return {
      onEvent: () => () => {},
      onError: () => {},
      close: () => {},
      send: (_data: TIn) => {},
      closeSend: () => {},
    };
  },
};

void meta;
void transport;
void encodePathSegment("a");
void encodeMultiSegmentPath("users/a");
