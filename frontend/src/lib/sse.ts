// Shared by use-weather-stream.ts and use-forecast-stream.ts — both consume
// backend SSE endpoints with the same query shape and the same "error" event
// ambiguity (a server-sent named frame with .data vs. a connection failure
// with none).

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

export function parseSSEErrorMessage(event: Event): string {
  const data = event instanceof MessageEvent ? event.data : null;
  if (!data) return 'connection lost';
  try {
    return (JSON.parse(data) as { message?: string }).message ?? data;
  } catch {
    return data;
  }
}
