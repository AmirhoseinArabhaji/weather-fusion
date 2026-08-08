'use client';

import { useEffect, useState } from 'react';

export type LocationSource = 'saved' | 'gps' | 'ip' | 'manual';

export interface ResolvedLocation {
  city: string;
  lat: number;
  lon: number;
  source: LocationSource;
}

export interface LocationState {
  location: ResolvedLocation | null;
  loading: boolean;
  error: string | null;
  setManualLocation: (loc: { city: string; lat: number; lon: number }) => void;
}

const STORAGE_KEY = 'weather-fusion:location';

function readSaved(): ResolvedLocation | null {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as ResolvedLocation) : null;
  } catch {
    return null;
  }
}

function save(loc: ResolvedLocation) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(loc));
  } catch {
    // private mode / quota exceeded — persistence is a nicety, not required
  }
}

/**
 * Resolves the user's location for the dashboard's default city:
 * saved choice -> browser Geolocation (GPS/wifi, permission-gated) ->
 * IP geolocation (silent, city-level) -> manual override via setManualLocation.
 *
 * GPS only yields coordinates, not a city name — `city` stays empty for a
 * 'gps' result until the weather API response resolves it (it returns the
 * geocoded city for whatever lat/lon it was given).
 */
export function useLocation(): LocationState {
  const [location, setLocation] = useState<ResolvedLocation | null>(() => readSaved());
  const [loading, setLoading] = useState(() => readSaved() === null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (location) {
      // already resolved from a saved value via lazy init above
      return;
    }

    let cancelled = false;

    const tryIP = async () => {
      try {
        const res = await fetch('https://ipapi.co/json/');
        if (!res.ok) throw new Error(`ip lookup failed: ${res.status}`);
        const data = await res.json();
        if (cancelled) return;
        if (data.latitude == null || data.longitude == null) {
          throw new Error('ip lookup returned no coordinates');
        }
        const resolved: ResolvedLocation = {
          city: [data.city, data.region].filter(Boolean).join(', ') || data.country_name || '',
          lat: data.latitude,
          lon: data.longitude,
          source: 'ip',
        };
        setLocation(resolved);
        save(resolved);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'location lookup failed');
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    if (typeof navigator !== 'undefined' && navigator.geolocation) {
      navigator.geolocation.getCurrentPosition(
        (pos) => {
          if (cancelled) return;
          const resolved: ResolvedLocation = {
            city: '',
            lat: pos.coords.latitude,
            lon: pos.coords.longitude,
            source: 'gps',
          };
          setLocation(resolved);
          save(resolved);
          setLoading(false);
        },
        () => {
          // denied, unavailable, or timed out — fall back to IP geolocation
          void tryIP();
        },
        { timeout: 8000, maximumAge: 300_000 },
      );
    } else {
      void tryIP();
    }

    return () => {
      cancelled = true;
    };
  }, [location]);

  const setManualLocation = (loc: { city: string; lat: number; lon: number }) => {
    const resolved: ResolvedLocation = { ...loc, source: 'manual' };
    setLocation(resolved);
    save(resolved);
  };

  return { location, loading, error, setManualLocation };
}
