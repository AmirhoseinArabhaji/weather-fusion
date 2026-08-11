'use client';

import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useLocationSearch } from '@/hooks/use-location-search';
import type { LocationMatch } from '@/lib/weather-types';
import type { ThemeTokens } from './weather-dashboard';

export interface LocationAutocompleteProps {
  t: ThemeTokens;
  accent: string;
  placeholder: string;
  hasError: boolean;
  /** Pulses the status dot — background GPS/IP location resolution in progress. */
  loading: boolean;
  /** A dropdown pick — exact coordinates already known, no further resolution needed. */
  onSelect: (match: LocationMatch) => void;
  /** Enter/Set with no suggestion highlighted — just the typed text, backend resolves it. */
  onSubmitFreeText: (text: string) => void;
}

/**
 * City input with a debounced autocomplete dropdown (backed by
 * GET /api/v1/locations/search, see use-location-search). Works either way:
 * pick a suggestion for an exact match, or type a city and hit Set/Enter
 * without picking anything — both paths are always available, never gated
 * behind the other.
 */
export default function LocationAutocomplete({
  t,
  accent,
  placeholder,
  hasError,
  loading,
  onSelect,
  onSubmitFreeText,
}: LocationAutocompleteProps) {
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(false);
  const [highlighted, setHighlighted] = useState(-1);
  const containerRef = useRef<HTMLDivElement>(null);

  const { results } = useLocationSearch(open ? query : '');

  useEffect(() => {
    function onClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', onClickOutside);
    return () => document.removeEventListener('mousedown', onClickOutside);
  }, []);

  const selectMatch = (m: LocationMatch) => {
    onSelect(m);
    setQuery('');
    setOpen(false);
    setHighlighted(-1);
  };

  const submitFreeText = () => {
    const trimmed = query.trim();
    if (!trimmed) return;
    onSubmitFreeText(trimmed);
    setQuery('');
    setOpen(false);
    setHighlighted(-1);
  };

  const confirmSelection = () => {
    if (highlighted >= 0 && highlighted < results.length) {
      selectMatch(results[highlighted]);
    } else {
      submitFreeText();
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (!open || results.length === 0) {
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setHighlighted((h) => (h + 1) % results.length);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setHighlighted((h) => (h <= 0 ? results.length - 1 : h - 1));
    } else if (e.key === 'Escape') {
      setOpen(false);
    }
  };

  return (
    <div ref={containerRef} style={{ position: 'relative' }}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          confirmSelection();
        }}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          background: t.card,
          border: `1px solid ${hasError ? 'oklch(0.55 0.16 25)' : t.border}`,
          borderRadius: 10,
          padding: '6px 8px 6px 14px',
        }}
      >
        <div
          style={{
            width: 7,
            height: 7,
            borderRadius: '50%',
            background: hasError ? 'oklch(0.55 0.16 25)' : accent,
            animation: loading ? 'pulseDot 1s ease-in-out infinite' : 'none',
            flexShrink: 0,
          }}
        />
        <input
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            setHighlighted(-1);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          style={{ border: 'none', outline: 'none', background: 'transparent', fontSize: 14, color: t.text, width: 220 }}
        />
        <button
          type="submit"
          style={{
            border: 'none',
            cursor: 'pointer',
            fontSize: 12.5,
            fontWeight: 600,
            padding: '6px 10px',
            borderRadius: 7,
            background: accent,
            color: 'white',
          }}
        >
          Set
        </button>
      </form>

      {open && results.length > 0 && (
        <div
          style={{
            position: 'absolute',
            top: 'calc(100% + 6px)',
            left: 0,
            right: 0,
            background: t.card,
            border: `1px solid ${t.border}`,
            borderRadius: 10,
            overflow: 'hidden',
            zIndex: 20,
            boxShadow: '0 8px 24px rgba(0, 0, 0, 0.16)',
          }}
        >
          {results.map((r, i) => (
            <div
              key={`${r.name}-${r.lat}-${r.lon}`}
              // mousedown (not click) fires before the input would blur, and
              // preventDefault keeps focus on the input rather than the browser
              // shifting it to this div — selection registers reliably either way.
              onMouseDown={(e) => {
                e.preventDefault();
                selectMatch(r);
              }}
              onMouseEnter={() => setHighlighted(i)}
              style={{
                padding: '9px 14px',
                cursor: 'pointer',
                fontSize: 13.5,
                background: highlighted === i ? t.trackBg : 'transparent',
                borderBottom: i < results.length - 1 ? `1px solid ${t.borderSoft}` : 'none',
              }}
            >
              <span style={{ color: t.text }}>{r.name}</span>
              {(r.admin1 || r.country) && (
                <span style={{ color: t.text3 }}> · {[r.admin1, r.country].filter(Boolean).join(', ')}</span>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
