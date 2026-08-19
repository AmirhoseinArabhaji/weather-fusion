// Shared by use-weather-stream.ts and use-forecast-stream.ts — both consume
// backend SSE endpoints with the same query shape and the same need to see
// the real HTTP status/body when the backend rejects the request before the
// stream even opens (validation, rate limit, no providers). EventSource
// can't expose that (no status/header/body access on a failed connection),
// so this parses the stream by hand over fetch()/ReadableStream instead.

export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

export interface StreamParams {
  city?: string;
  lat?: number;
  lon?: number;
  units?: string;
  days?: number;
}

export function buildStreamQuery(params: StreamParams): string {
  const query = new URLSearchParams();
  if (params.city) query.set('city', params.city);
  if (params.lat != null) query.set('lat', String(params.lat));
  if (params.lon != null) query.set('lon', String(params.lon));
  if (params.units) query.set('units', params.units);
  if (params.days != null) query.set('days', String(params.days));
  return query.toString();
}

export interface SSEHttpError {
  status: number;
  code?: string;
  message: string;
}

export interface SSEHandlers {
  onEvent: (event: string, data: string) => void;
  onHttpError: (err: SSEHttpError) => void;
  onNetworkError: () => void;
}

// Matches the backend's response.Envelope shape: {"success":false,"error":{"code":...,"message":...}}
interface ErrorEnvelope {
  error?: { code?: string; message?: string };
}

/**
 * Reads a `text/event-stream` response by hand (gin's c.SSEvent format:
 * "event: name\ndata: json\n\n"), so a non-2xx response's real status,
 * Retry-After header, and JSON error body are all available — none of
 * which EventSource exposes.
 */
export async function streamSSE(url: string, handlers: SSEHandlers, signal: AbortSignal): Promise<void> {
  let res: Response;
  try {
    res = await fetch(url, { signal, headers: { Accept: 'text/event-stream' } });
  } catch {
    if (!signal.aborted) handlers.onNetworkError();
    return;
  }

  if (!res.ok) {
    let code: string | undefined;
    let message = `request failed (${res.status})`;
    try {
      const body = (await res.json()) as ErrorEnvelope;
      code = body.error?.code;
      message = body.error?.message ?? message;
    } catch {
      // non-JSON error body — keep the generic status-based message
    }
    handlers.onHttpError({ status: res.status, code, message });
    return;
  }

  const reader = res.body?.getReader();
  if (!reader) {
    handlers.onNetworkError();
    return;
  }

  const decoder = new TextDecoder();
  let buffer = '';
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });

      let frameEnd: number;
      while ((frameEnd = buffer.indexOf('\n\n')) !== -1) {
        const frame = buffer.slice(0, frameEnd);
        buffer = buffer.slice(frameEnd + 2);

        let event = 'message';
        const dataLines: string[] = [];
        for (const line of frame.split('\n')) {
          if (line.startsWith('event:')) event = line.slice(6).trim();
          else if (line.startsWith('data:')) dataLines.push(line.slice(5).trim());
        }
        if (dataLines.length > 0) handlers.onEvent(event, dataLines.join('\n'));
      }
    }
  } catch {
    if (!signal.aborted) handlers.onNetworkError();
  }
}
