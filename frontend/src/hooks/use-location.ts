'use client';

import { useEffect, useState } from 'react';

export type LocationSource = 'saved' | 'gps' | 'ip' | 'manual';

export interface ResolvedLocation {
  city: string;
  // null for a manually-typed city — there's no client-side geocoding, so no
  // coordinates exist until the backend resolves the name itself.
  lat: number | null;
  lon: number | null;
  source: LocationSource;
}

export interface LocationState {
  location: ResolvedLocation | null;
  loading: boolean;
  error: string | null;
  // coords omitted (free-text submit, no known coordinates yet — the backend
  // resolves the name itself) or given (a specific autocomplete pick, so no
  // further resolution ambiguity).
  setManualLocation: (city: string, coords?: { lat: number; lon: number }) => void;
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
  // Starts identical on server and client (localStorage doesn't exist during
  // SSR) — reading a saved value happens inside the effect below instead of a
  // lazy initializer, otherwise the client's first paint disagrees with the
  // server's and React gives up reconciling that subtree (hydration mismatch).
  const [location, setLocation] = useState<ResolvedLocation | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (location) {
      return;
    }

    let cancelled = false;

    // Deferred to a microtask (rather than called synchronously here) so this
    // matches the same async-callback shape as the GPS/IP paths below —
    // setState belongs in a callback reacting to an external read, not
    // directly in the effect body.
    const resolveSaved = async (): Promise<boolean> => {
      const saved = readSaved();
      if (!saved) return false;
      if (!cancelled) {
        setLocation(saved);
        setLoading(false);
      }
      return true;
    };

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

    void resolveSaved().then((found) => {
      if (found || cancelled) return;

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
    });

    return () => {
      cancelled = true;
    };
  }, [location]);

  const setManualLocation = (city: string, coords?: { lat: number; lon: number }) => {
    const resolved: ResolvedLocation = {
      city,
      lat: coords?.lat ?? null,
      lon: coords?.lon ?? null,
      source: 'manual',
    };
    setLocation(resolved);
    save(resolved);
    setError(null);
  };

  return { location, loading, error, setManualLocation };
}
