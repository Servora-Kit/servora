export interface TransportMeta {
  service: string;
  method: string;
}

export interface ServerStream<T> {
  onEvent(listener: (data: T) => void): () => void;
  onError(handler: (error: Error) => void): void;
  close(): void;
}

export interface DuplexStream<TIn, TOut> extends ServerStream<TOut> {
  send(data: TIn): void;
  closeSend(): void;
}

export interface ClientTransport {
  unary<T>(
    path: string,
    method: string,
    body: null | string,
    meta: TransportMeta,
  ): Promise<T>;
  serverStream<T>(path: string, meta: TransportMeta): ServerStream<T>;
  duplexStream<TIn, TOut>(
    path: string,
    meta: TransportMeta,
    encode?: (data: TIn) => unknown,
  ): DuplexStream<TIn, TOut>;
}

export function encodePathSegment(value: unknown): string {
  return encodeURIComponent(String(value)).replace(
    /[!'()*]/g,
    (character) => `%${character.charCodeAt(0).toString(16).toUpperCase()}`,
  );
}

export function encodeMultiSegmentPath(value: unknown): string {
  return String(value).split("/").map(encodePathSegment).join("/");
}
