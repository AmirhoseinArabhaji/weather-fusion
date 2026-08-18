'use client';

import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useLocationSearch } from '@/hooks/use-location-search';
import type { LocationMatch } from '@/lib/weather-types';
import type { ThemeTokens } from './weather-dashboard';

export interface LocationAutocompleteProps {
  t: ThemeTokens;
  dark: boolean;
  placeholder: string;
  hasError: boolean;
  /** Pulses the status dot — background GPS/IP location resolution in progress. */
  loading: boolean;
  /** A dropdown pick — exact coordinates already known, no further resolution needed. */
  onSelect: (match: LocationMatch) => void;
  /** Enter with no suggestion highlighted — just the typed text, backend resolves it. */
  onSubmitFreeText: (text: string) => void;
  /** Floor on the input's width — the default (250) is what pushes nav controls
   * off-screen on a narrow viewport, so callers on small screens pass 0. */
  minWidth?: number;
}

const DOT = 'oklch(0.68 0.16 235)';
const ERROR_DOT = 'oklch(0.62 0.19 25)';

/**
 * City input with a debounced autocomplete dropdown (backed by
 * GET /api/v1/locations/search, see use-location-search). Works either way:
 * pick a suggestion for an exact match, or type a city and hit Enter (or the
 * search button) without picking anything — both paths are always available,
 * never gated behind the other.
 */
export default function LocationAutocomplete({
  t,
  dark,
  placeholder,
  hasError,
  loading,
  onSelect,
  onSubmitFreeText,
  minWidth = 250,
}: LocationAutocompleteProps) {
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(false);
  const [highlighted, setHighlighted] = useState(-1);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

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

  const dotColor = hasError ? ERROR_DOT : DOT;

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
          gap: 9,
          background: t.glass,
          border: `1px solid ${hasError ? ERROR_DOT : t.border}`,
          borderRadius: 12,
          padding: '9px 10px 9px 15px',
          minWidth,
        }}
      >
        <div
          style={{
            width: 6,
            height: 6,
            borderRadius: '50%',
            background: dotColor,
            boxShadow: `0 0 8px ${dotColor}`,
            animation: loading ? 'pulseDot 1.1s ease-in-out infinite' : 'none',
            flexShrink: 0,
          }}
        />
        <input
          ref={inputRef}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            setHighlighted(-1);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          style={{
            border: 'none',
            outline: 'none',
            background: 'transparent',
            color: t.text,
            fontSize: 14,
            fontWeight: 500,
            width: '100%',
            fontFamily: "var(--font-space-grotesk), 'Space Grotesk', sans-serif",
          }}
        />
        <button
          type="button"
          onClick={() => {
            if (query.trim()) {
              confirmSelection();
            } else {
              inputRef.current?.focus();
              setOpen(true);
            }
          }}
          title="Search"
          aria-label="Search"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 22,
            height: 22,
            color: t.text3,
            background: 'transparent',
            border: 'none',
            borderRadius: 6,
            padding: 0,
            flexShrink: 0,
            cursor: 'pointer',
          }}
        >
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
            <circle cx="11" cy="11" r="7" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
        </button>
      </form>

      {open && results.length > 0 && (
        <div
          style={{
            position: 'absolute',
            top: 'calc(100% + 8px)',
            left: 0,
            right: 0,
            background: t.glass,
            border: `1px solid ${t.border}`,
            borderRadius: 12,
            overflow: 'hidden',
            zIndex: 20,
            boxShadow: dark ? '0 16px 40px rgba(0, 0, 0, 0.5)' : '0 16px 40px rgba(20, 30, 50, 0.18)',
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
                padding: '10px 15px',
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
