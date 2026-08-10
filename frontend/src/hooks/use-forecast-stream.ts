'use client';

import { useEffect, useState } from 'react';
import type { ConsensusHourly, DailyForecast, DailyProviderEvent, HourlyProviderEvent } from '@/lib/weather-types';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';

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
}

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
  const [state, setState] = useState<ForecastStreamState>({
    status: 'idle',
    dailyEvents: [],
    hourlyEvents: [],
    daily: null,
    dailyError: null,
    hourly: null,
    hourlyError: null,
    error: null,
  });

  const city = params?.city;
  const lat = params?.lat;
  const lon = params?.lon;
  const units = params?.units;
  const days = params?.days;

  useEffect(() => {
    if (!city && (lat == null || lon == null)) {
      return;
    }

    const query = new URLSearchParams();
    if (city) query.set('city', city);
    if (lat != null) query.set('lat', String(lat));
    if (lon != null) query.set('lon', String(lon));
    if (units) query.set('units', units);
    if (days != null) query.set('days', String(days));

    const source = new EventSource(`${API_BASE_URL}/api/v1/weather/forecast?${query.toString()}`);

    source.addEventListener('open', () => {
      setState({
        status: 'connecting',
        dailyEvents: [],
        hourlyEvents: [],
        daily: null,
        dailyError: null,
        hourly: null,
        hourlyError: null,
        error: null,
      });
    });

    source.addEventListener('provider_daily', (event) => {
      const parsed = JSON.parse((event as MessageEvent).data) as DailyProviderEvent;
      setState((s) => ({ ...s, status: 'streaming', dailyEvents: [...s.dailyEvents, parsed] }));
    });

    source.addEventListener('provider_hourly', (event) => {
      const parsed = JSON.parse((event as MessageEvent).data) as HourlyProviderEvent;
      setState((s) => ({ ...s, status: 'streaming', hourlyEvents: [...s.hourlyEvents, parsed] }));
    });

    source.addEventListener('daily', (event) => {
      const parsed = JSON.parse((event as MessageEvent).data) as DailyForecast[] | { error: string };
      if (Array.isArray(parsed)) {
        setState((s) => ({ ...s, daily: parsed }));
      } else {
        setState((s) => ({ ...s, dailyError: parsed.error }));
      }
    });

    source.addEventListener('hourly', (event) => {
      const parsed = JSON.parse((event as MessageEvent).data) as ConsensusHourly[] | { error: string };
      if (Array.isArray(parsed)) {
        setState((s) => ({ ...s, status: 'done', hourly: parsed }));
      } else {
        setState((s) => ({ ...s, status: 'done', hourlyError: parsed.error }));
      }
      source.close();
    });

    // Same disambiguation as useWeatherStream: a server-sent named "error"
    // frame (has .data) vs. a connection-level failure (plain Event, no .data).
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
  }, [city, lat, lon, units, days]);

  return state;
}
