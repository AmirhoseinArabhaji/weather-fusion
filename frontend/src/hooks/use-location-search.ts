'use client';

import { useEffect, useState } from 'react';
import type { LocationMatch } from '@/lib/weather-types';
import { API_BASE_URL } from '@/lib/sse';

const DEBOUNCE_MS = 300;
const MIN_QUERY_LENGTH = 2;

export interface LocationSearchState {
  results: LocationMatch[];
  loading: boolean;
}

/**
 * Debounced place-name search backed by GET /api/v1/locations/search
 * (backend-proxied Photon geocoding — see backend/internal/geocoding).
 * Empty/short queries return no results without hitting the network.
 */
export function useLocationSearch(query: string): LocationSearchState {
  const [results, setResults] = useState<LocationMatch[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const trimmed = query.trim();
    let cancelled = false;

    // setState deferred into this async body (not called synchronously at
    // the top of the effect) — same shape as use-location.ts's resolveSaved.
    const run = async () => {
      if (trimmed.length < MIN_QUERY_LENGTH) {
        if (!cancelled) {
          setResults([]);
          setLoading(false);
        }
        return;
      }

      if (!cancelled) setLoading(true);
      await new Promise((resolve) => setTimeout(resolve, DEBOUNCE_MS));
      if (cancelled) return;

      try {
        const res = await fetch(`${API_BASE_URL}/api/v1/locations/search?q=${encodeURIComponent(trimmed)}`);
        const body: { data?: LocationMatch[] } = await res.json();
        if (!cancelled) setResults(body.data ?? []);
      } catch {
        if (!cancelled) setResults([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    void run();

    return () => {
      cancelled = true;
    };
  }, [query]);

  return { results, loading };
}
