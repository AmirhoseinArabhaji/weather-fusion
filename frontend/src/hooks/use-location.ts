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

const IP_LOOKUP_TIMEOUT_MS = 5000;

// Single keyless provider — ipwho.is over ipapi.co: same nominal free-tier
// limit on paper, but ipapi.co actually hit real rate-limit errors repeatedly
// in practice while ipwho.is never failed once. A timeout still matters even
// with one provider — a hang (not just an error response) would otherwise
// block location resolution forever.
async function fetchIPLocation(): Promise<{ city: string; lat: number; lon: number }> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), IP_LOOKUP_TIMEOUT_MS);
  try {
    const res = await fetch('https://ipwho.is/', { signal: controller.signal });
    if (!res.ok) throw new Error(`ipwho.is: ${res.status}`);
    const data = (await res.json()) as Record<string, unknown>;
    if (!data.success || data.latitude == null || data.longitude == null) {
      throw new Error('ipwho.is: no coordinates');
    }
    return {
      city: [data.city, data.region].filter(Boolean).join(', ') || (data.country as string) || '',
      lat: data.latitude as number,
      lon: data.longitude as number,
    };
  } finally {
    clearTimeout(timer);
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
        const { city, lat, lon } = await fetchIPLocation();
        if (cancelled) return;
        const resolved: ResolvedLocation = { city, lat, lon, source: 'ip' };
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
