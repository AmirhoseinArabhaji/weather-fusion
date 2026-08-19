'use client';

import { useEffect, useState } from 'react';
import type { ConsensusHourly, DailyForecast, DailyProviderEvent, HourlyProviderEvent } from '@/lib/weather-types';
import { API_BASE_URL, buildStreamQuery, streamSSE } from '@/lib/sse';

export interface ForecastStreamParams {
  city?: string;
  lat?: number;
  lon?: number;
  units?: 'standard' | 'metric' | 'imperial';
  days?: number;
}

export type ForecastStreamStatus = 'idle' | 'connecting' | 'streaming' | 'done' | 'error';

export interface ForecastStreamState {
  status: ForecastStreamStatus;
  dailyEvents: DailyProviderEvent[];
  hourlyEvents: HourlyProviderEvent[];
  daily: DailyForecast[] | null;
  dailyError: string | null;
  hourly: ConsensusHourly[] | null;
  hourlyError: string | null;
  error: string | null;
  rateLimited: boolean;
}

const IDLE_STATE: ForecastStreamState = {
  status: 'idle',
  dailyEvents: [],
  hourlyEvents: [],
  daily: null,
  dailyError: null,
  hourly: null,
  hourlyError: null,
  error: null,
  rateLimited: false,
};

/**
 * Consumes GET /api/v1/weather/forecast via SSE: "provider_daily"/"provider_hourly"
 * events as each provider resolves, then merged "daily" and "hourly" consensus
 * events. The backend always sends daily before hourly, so "hourly" is the
 * reliable close signal here (mirrors how useWeatherStream waits for "summary",
 * not "consensus").
 *
 * Pass null to hold off connecting (e.g. while location is still resolving).
 */
export function useForecastStream(params: ForecastStreamParams | null): ForecastStreamState {
  const [state, setState] = useState<ForecastStreamState>(IDLE_STATE);

  const city = params?.city;
  const lat = params?.lat;
  const lon = params?.lon;
  const units = params?.units;
  const days = params?.days;

  useEffect(() => {
    if (!city && (lat == null || lon == null)) {
      return;
    }

    const controller = new AbortController();
    const query = buildStreamQuery({ city, lat, lon, units, days });

    setState({ ...IDLE_STATE, status: 'connecting' });

    void streamSSE(
      `${API_BASE_URL}/api/v1/weather/forecast?${query}`,
      {
        onEvent: (event, data) => {
          if (event === 'provider_daily') {
            const parsed = JSON.parse(data) as DailyProviderEvent;
            setState((s) => ({ ...s, status: 'streaming', dailyEvents: [...s.dailyEvents, parsed] }));
          } else if (event === 'provider_hourly') {
            const parsed = JSON.parse(data) as HourlyProviderEvent;
            setState((s) => ({ ...s, status: 'streaming', hourlyEvents: [...s.hourlyEvents, parsed] }));
          } else if (event === 'daily') {
            const parsed = JSON.parse(data) as DailyForecast[] | { error: string };
            if (Array.isArray(parsed)) {
              setState((s) => ({ ...s, daily: parsed }));
            } else {
              setState((s) => ({ ...s, dailyError: parsed.error }));
            }
          } else if (event === 'hourly') {
            const parsed = JSON.parse(data) as ConsensusHourly[] | { error: string };
            if (Array.isArray(parsed)) {
              setState((s) => ({ ...s, status: 'done', hourly: parsed }));
            } else {
              setState((s) => ({ ...s, status: 'done', hourlyError: parsed.error }));
            }
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
  }, [city, lat, lon, units, days]);

  return state;
}
