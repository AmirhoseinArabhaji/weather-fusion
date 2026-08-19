'use client';

import { useEffect, useState } from 'react';
import type { ConsensusResult, ProviderEvent, SummaryEvent } from '@/lib/weather-types';
import { API_BASE_URL, buildStreamQuery, streamSSE } from '@/lib/sse';

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
  rateLimited: boolean;
}

const IDLE_STATE: WeatherStreamState = {
  status: 'idle',
  providerEvents: [],
  consensus: null,
  summary: null,
  summaryError: null,
  error: null,
  rateLimited: false,
};

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
  const [state, setState] = useState<WeatherStreamState>(IDLE_STATE);

  const city = params?.city;
  const lat = params?.lat;
  const lon = params?.lon;
  const units = params?.units;

  useEffect(() => {
    if (!city && (lat == null || lon == null)) {
      return;
    }

    const controller = new AbortController();
    const query = buildStreamQuery({ city, lat, lon, units });

    setState({ ...IDLE_STATE, status: 'connecting' });

    void streamSSE(
      `${API_BASE_URL}/api/v1/weather/current?${query}`,
      {
        onEvent: (event, data) => {
          if (event === 'provider') {
            const parsed = JSON.parse(data) as ProviderEvent;
            setState((s) => ({ ...s, status: 'streaming', providerEvents: [...s.providerEvents, parsed] }));
          } else if (event === 'consensus') {
            const parsed = JSON.parse(data) as ConsensusResult;
            // Not done yet — the summary event still has to arrive.
            setState((s) => ({ ...s, consensus: parsed }));
          } else if (event === 'summary') {
            const parsed = JSON.parse(data) as SummaryEvent;
            setState((s) => ({ ...s, status: 'done', summary: parsed.summary ?? null, summaryError: parsed.error ?? null }));
          } else if (event === 'error') {
            const parsed = JSON.parse(data) as { message?: string };
            setState((s) => ({ ...s, status: 'error', error: parsed.message ?? data }));
          }
        },
        onHttpError: (err) => {
          setState((s) => ({
            ...s,
            status: 'error',
            error: err.message,
            rateLimited: err.code === 'RATE_LIMITED',
          }));
        },
        onNetworkError: () => {
          setState((s) => ({ ...s, status: 'error', error: 'connection lost' }));
        },
      },
      controller.signal,
    );

    return () => controller.abort();
  }, [city, lat, lon, units]);

  return state;
}
