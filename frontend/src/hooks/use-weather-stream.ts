'use client';

import { useEffect, useState } from 'react';
import type { ConsensusResult, ProviderEvent, SummaryEvent } from '@/lib/weather-types';
import { buildStreamQuery, parseSSEErrorMessage } from '@/lib/sse';

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
  summary: string | null;
  summaryError: string | null;
  error: string | null;
}

/**
 * Consumes GET /api/v1/weather/current via SSE: a "provider" event per source
 * as it resolves, a "consensus" event with the merged numeric result, then a
 * separate "summary" event once the LLM interpretation is ready (it runs
 * after consensus is already flushed, so it lands a beat later — that's the
 * connection's actual close signal, not "consensus").
 *
 * Pass null to hold off connecting (e.g. while location is still resolving).
 */
export function useWeatherStream(params: WeatherStreamParams | null): WeatherStreamState {
  const [state, setState] = useState<WeatherStreamState>({
    status: 'idle',
    providerEvents: [],
    consensus: null,
    summary: null,
    summaryError: null,
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

    const query = buildStreamQuery({ city, lat, lon, units });
    const source = new EventSource(`${API_BASE_URL}/api/v1/weather/current?${query}`);

    // Connection just opened — clear out whatever the previous params' run left behind.
    source.addEventListener('open', () => {
      setState({ status: 'connecting', providerEvents: [], consensus: null, summary: null, summaryError: null, error: null });
    });

    source.addEventListener('provider', (event) => {
      const parsed = JSON.parse((event as MessageEvent).data) as ProviderEvent;
      setState((s) => ({ ...s, status: 'streaming', providerEvents: [...s.providerEvents, parsed] }));
    });

    source.addEventListener('consensus', (event) => {
      const parsed = JSON.parse((event as MessageEvent).data) as ConsensusResult;
      // Not done yet — the summary event still has to arrive.
      setState((s) => ({ ...s, consensus: parsed }));
    });

    source.addEventListener('summary', (event) => {
      const parsed = JSON.parse((event as MessageEvent).data) as SummaryEvent;
      setState((s) => ({ ...s, status: 'done', summary: parsed.summary ?? null, summaryError: parsed.error ?? null }));
      source.close();
    });

    // The "error" listener catches two distinct things: a server-sent named
    // "error" SSE frame (has .data, e.g. "no providers responded") and a
    // connection-level failure (plain Event, no .data). Both use the same
    // EventSource listener per spec, so they're disambiguated in parseSSEErrorMessage.
    source.addEventListener('error', (event) => {
      setState((s) => ({ ...s, status: 'error', error: parseSSEErrorMessage(event) }));
      source.close();
    });

    return () => source.close();
  }, [city, lat, lon, units]);

  return state;
}
