'use client';

import { useEffect, useState } from 'react';
import type { ConsensusResult, ProviderEvent } from '@/lib/weather-types';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

export interface WeatherStreamParams {
  city?: string;
  lat?: number;
  lon?: number;
  units?: 'standard' | 'metric' | 'imperial';
}

export type WeatherStreamStatus = 'idle' | 'connecting' | 'streaming' | 'done' | 'error';

export interface WeatherStreamState {
  status: WeatherStreamStatus;
  providerEvents: ProviderEvent[];
  consensus: ConsensusResult | null;
  error: string | null;
}

/**
 * Consumes GET /api/v1/weather/current via SSE: a "provider" event per source
 * as it resolves, then one "consensus" event with the merged result.
 *
 * Pass null to hold off connecting (e.g. while location is still resolving).
 */
export function useWeatherStream(params: WeatherStreamParams | null): WeatherStreamState {
  const [state, setState] = useState<WeatherStreamState>({
    status: 'idle',
    providerEvents: [],
    consensus: null,
    error: null,
  });

  const city = params?.city;
  const lat = params?.lat;
  const lon = params?.lon;
  const units = params?.units;

  useEffect(() => {
    if (!city && (lat == null || lon == null)) {
      return;
    }

    const query = new URLSearchParams();
    if (city) query.set('city', city);
    if (lat != null) query.set('lat', String(lat));
    if (lon != null) query.set('lon', String(lon));
    if (units) query.set('units', units);

    const source = new EventSource(`${API_BASE_URL}/api/v1/weather/current?${query.toString()}`);

    // Connection just opened — clear out whatever the previous params' run left behind.
    source.addEventListener('open', () => {
      setState({ status: 'connecting', providerEvents: [], consensus: null, error: null });
    });

    source.addEventListener('provider', (event) => {
      const parsed = JSON.parse((event as MessageEvent).data) as ProviderEvent;
      setState((s) => ({ ...s, status: 'streaming', providerEvents: [...s.providerEvents, parsed] }));
    });

    source.addEventListener('consensus', (event) => {
      const parsed = JSON.parse((event as MessageEvent).data) as ConsensusResult;
      setState((s) => ({ ...s, status: 'done', consensus: parsed }));
      source.close();
    });

    // The "error" listener catches two distinct things: a server-sent named
    // "error" SSE frame (has .data, e.g. "no providers responded") and a
    // connection-level failure (plain Event, no .data). Both use the same
    // EventSource listener per spec, so they're disambiguated here.
    source.addEventListener('error', (event) => {
      const data = event instanceof MessageEvent ? event.data : null;
      let message = 'connection lost';
      if (data) {
        try {
          message = (JSON.parse(data) as { message?: string }).message ?? data;
        } catch {
          message = data;
        }
      }
      setState((s) => ({ ...s, status: 'error', error: message }));
      source.close();
    });

    return () => source.close();
  }, [city, lat, lon, units]);

  return state;
}
