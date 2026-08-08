'use client';

import { useState, type CSSProperties } from 'react';
import { useLocation } from '@/hooks/use-location';
import { useWeatherStream } from '@/hooks/use-weather-stream';

type Units = 'C' | 'F';
type Theme = 'light' | 'dark';
type ProviderStatus = 'unavailable' | 'loading' | 'success' | 'failed';

interface HourReading {
  hourLabel: string;
  lowC: number;
  highC: number;
  rain: number;
}

interface DayReading {
  dayLabel: string;
  lowC: number;
  highC: number;
  rain: number;
}

const SPACE_GROTESK = "var(--font-space-grotesk), 'Space Grotesk', sans-serif";
const ACCENT = 'oklch(0.55 0.14 235)';

// Assigned to real providers by arrival order — provider identity/count comes
// from the backend now, not a fixed list, so colors can't be keyed by name.
const PROVIDER_PALETTE = [
  'oklch(0.55 0.14 235)',
  'oklch(0.6 0.1 150)',
  'oklch(0.68 0.12 70)',
  'oklch(0.58 0.1 320)',
  'oklch(0.58 0.11 20)',
];

// Demo data — the backend has no forecast or accuracy-tracking endpoint
// wired up yet, so these stay illustrative until that exists.
const HOURLY_DATA: HourReading[] = [
  { hourLabel: '12PM', lowC: 15, highC: 18, rain: 30 },
  { hourLabel: '1PM', lowC: 16, highC: 19, rain: 35 },
  { hourLabel: '2PM', lowC: 16, highC: 19, rain: 55 },
  { hourLabel: '3PM', lowC: 15, highC: 19, rain: 65 },
  { hourLabel: '4PM', lowC: 15, highC: 18, rain: 75 },
  { hourLabel: '5PM', lowC: 14, highC: 18, rain: 80 },
  { hourLabel: '6PM', lowC: 14, highC: 17, rain: 70 },
  { hourLabel: '7PM', lowC: 13, highC: 16, rain: 50 },
];

const DAILY_DATA: DayReading[] = [
  { dayLabel: 'Fri', lowC: 13, highC: 18, rain: 65 },
  { dayLabel: 'Sat', lowC: 14, highC: 20, rain: 20 },
  { dayLabel: 'Sun', lowC: 13, highC: 19, rain: 10 },
  { dayLabel: 'Mon', lowC: 12, highC: 17, rain: 45 },
  { dayLabel: 'Tue', lowC: 11, highC: 16, rain: 55 },
];

const ACCURACY: { name: string; pct: number }[] = [
  { name: 'openweather', pct: 91 },
  { name: 'weatherapi', pct: 84 },
];

interface ThemeTokens {
  bg: string;
  card: string;
  border: string;
  borderSoft: string;
  trackBg: string;
  text: string;
  text2: string;
  text3: string;
}

function themeTokens(dark: boolean): ThemeTokens {
  return dark
    ? {
        bg: 'oklch(0.19 0.01 240)',
        card: 'oklch(0.235 0.012 240)',
        border: 'oklch(0.32 0.012 240)',
        borderSoft: 'oklch(0.29 0.012 240)',
        trackBg: 'oklch(0.29 0.012 240)',
        text: 'oklch(0.93 0.005 240)',
        text2: 'oklch(0.8 0.008 240)',
        text3: 'oklch(0.62 0.01 240)',
      }
    : {
        bg: 'oklch(0.98 0.006 240)',
        card: 'oklch(1 0 0)',
        border: 'oklch(0.9 0.01 240)',
        borderSoft: 'oklch(0.94 0.006 240)',
        trackBg: 'oklch(0.93 0.01 240)',
        text: 'oklch(0.22 0.01 240)',
        text2: 'oklch(0.35 0.01 240)',
        text3: 'oklch(0.5 0.01 240)',
      };
}

interface Confidence {
  label: 'High' | 'Medium' | 'Low';
  soft: string;
  strong: string;
}

function confidenceFor(std: number, lowThresh: number, highThresh: number, dark: boolean): Confidence {
  if (std < lowThresh) {
    return {
      label: 'High',
      soft: dark ? 'oklch(0.3 0.05 150)' : 'oklch(0.95 0.03 150)',
      strong: dark ? 'oklch(0.78 0.13 150)' : 'oklch(0.4 0.11 150)',
    };
  }
  if (std < highThresh) {
    return {
      label: 'Medium',
      soft: dark ? 'oklch(0.32 0.06 70)' : 'oklch(0.95 0.04 70)',
      strong: dark ? 'oklch(0.78 0.13 70)' : 'oklch(0.45 0.13 70)',
    };
  }
  return {
    label: 'Low',
    soft: dark ? 'oklch(0.32 0.06 30)' : 'oklch(0.95 0.04 70)',
    strong: dark ? 'oklch(0.75 0.15 30)' : 'oklch(0.5 0.14 30)',
  };
}

interface StatusMeta {
  label: string;
  dotColor: string;
  labelColor: string;
  animation: string;
}

function statusMeta(status: ProviderStatus, dark: boolean, text3: string): StatusMeta {
  const map: Record<ProviderStatus, StatusMeta> = {
    unavailable: {
      label: 'Unavailable',
      dotColor: dark ? 'oklch(0.4 0.008 240)' : 'oklch(0.85 0.005 240)',
      labelColor: text3,
      animation: 'none',
    },
    loading: {
      label: 'Loading…',
      dotColor: ACCENT,
      labelColor: dark ? 'oklch(0.7 0.12 235)' : 'oklch(0.5 0.12 235)',
      animation: 'pulseDot 1s ease-in-out infinite',
    },
    success: {
      label: 'Loaded',
      dotColor: 'oklch(0.6 0.11 150)',
      labelColor: dark ? 'oklch(0.7 0.12 150)' : 'oklch(0.45 0.1 150)',
      animation: 'none',
    },
    failed: {
      label: 'Failed',
      dotColor: 'oklch(0.55 0.16 25)',
      labelColor: dark ? 'oklch(0.72 0.15 25)' : 'oklch(0.5 0.15 25)',
      animation: 'none',
    },
  };
  return map[status];
}

const mean = (arr: number[]) => arr.reduce((a, b) => a + b, 0) / arr.length;
const stddev = (arr: number[]) => {
  const m = mean(arr);
  return Math.sqrt(mean(arr.map((v) => (v - m) ** 2)));
};

const rainColorFor = (r: number, dark: boolean) =>
  r >= 65 ? ACCENT : r >= 35 ? 'oklch(0.68 0.12 70)' : dark ? 'oklch(0.55 0.01 240)' : 'oklch(0.75 0.02 240)';

function toggleButtonStyle(active: boolean, activeBg: string, activeColor: string, inactiveColor: string): CSSProperties {
  return {
    border: 'none',
    cursor: 'pointer',
    fontSize: '12.5px',
    fontWeight: 600,
    padding: '6px 10px',
    borderRadius: 7,
    background: active ? activeBg : 'transparent',
    color: active ? activeColor : inactiveColor,
  };
}

export interface WeatherDashboardProps {
  /** Overrides automatic location resolution (GPS -> IP -> saved). */
  city?: string;
}

export default function WeatherDashboard({ city: cityOverride }: WeatherDashboardProps) {
  const { location } = useLocation();
  const city = cityOverride ?? (location?.city || (location ? 'Current location' : 'San Francisco, CA'));

  const [units, setUnits] = useState<Units>('C');
  const [theme, setTheme] = useState<Theme>('light');

  const stream = useWeatherStream(location ? { lat: location.lat, lon: location.lon, units: 'metric' } : null);

  const dark = theme === 'dark';
  const t = themeTokens(dark);
  const activeBg = dark ? 'oklch(0.4 0.02 240)' : 'oklch(1 0 0)';
  const activeColor = dark ? 'oklch(0.95 0.005 240)' : 'oklch(0.2 0.01 240)';
  const inactiveColor = t.text3;

  const display = (c: number) => (units === 'F' ? `${Math.round((c * 9) / 5 + 32)}°F` : `${Math.round(c)}°C`);

  // Real providers, in arrival order — count and identity come from whatever
  // the backend actually has configured, not a fixed list.
  const providers = stream.providerEvents
    .filter((ev) => ev.status === 'ok' && ev.data)
    .map((ev, i) => {
      const data = ev.data!;
      return {
        name: ev.provider,
        color: PROVIDER_PALETTE[i % PROVIDER_PALETTE.length],
        tempC: data.temperature,
        feelsC: data.feels_like,
        condition: data.description || data.condition,
        rainPct: Math.round(data.precip_prob * 100),
        tempDisplay: display(data.temperature),
      };
    });

  const providerStatuses = stream.providerEvents.map((ev) => ({
    name: ev.provider,
    ...statusMeta(ev.status === 'ok' ? 'success' : 'failed', dark, t.text3),
  }));
  const gatheringLabel =
    stream.status === 'done' ? 'Data gathered' : stream.status === 'error' ? 'Error loading data' : 'Gathering data…';

  const temps = providers.map((p) => p.tempC);
  const rains = providers.map((p) => p.rainPct);
  const tempStd = temps.length > 0 ? stddev(temps) : 0;
  const rainStd = rains.length > 0 ? stddev(rains) : 0;

  const tempConfidence = confidenceFor(tempStd, 0.8, 1.5, dark);
  const rainConfidence = confidenceFor(rainStd, 12, 22, dark);

  const globalHigh = Math.max(...HOURLY_DATA.map((h) => h.highC));
  const globalLow = Math.min(...HOURLY_DATA.map((h) => h.lowC));
  const range = globalHigh - globalLow || 1;

  const hourly = HOURLY_DATA.map((h) => ({
    ...h,
    highDisplay: display(h.highC),
    lowDisplay: display(h.lowC),
    barHeight: Math.round(60 + ((h.highC - globalLow) / range) * 130),
    fillPct: Math.round(((h.highC - h.lowC) / range) * 100) + 35,
    rainColor: rainColorFor(h.rain, dark),
  }));

  const daily = DAILY_DATA.map((d) => ({
    ...d,
    highDisplay: display(d.highC),
    lowDisplay: display(d.lowC),
    rainColor: rainColorFor(d.rain, dark),
  }));

  const consensusTempDisplay = stream.consensus ? display(stream.consensus.temperature) : '—';
  const consensusFeelsDisplay = providers.length > 0 ? display(mean(providers.map((p) => p.feelsC))) : '—';
  const tempSpread = temps.length > 0 ? (Math.max(...temps) - Math.min(...temps)).toFixed(1) : '0.0';
  const rainRangeLabel = rains.length > 0 ? `${Math.min(...rains)}–${Math.max(...rains)}%` : '—';
  const rainDisagreementNote =
    rainConfidence.label === 'Low' ? 'Wide disagreement on rain — treat as uncertain' : 'Providers broadly agree on rain chance';

  const llmSummary =
    stream.status === 'error'
      ? (stream.error ?? 'Unable to load weather data.')
      : stream.summary
        ? stream.summary
        : stream.summaryError
          ? 'AI interpretation unavailable for this reading.'
          : stream.consensus
            ? 'Generating AI interpretation…'
            : 'Gathering provider data…';

  return (
    <div
      style={{
        minHeight: '100vh',
        background: t.bg,
        color: t.text,
        padding: '32px 40px 64px',
        transition: 'background 0.2s, color 0.2s',
        fontFamily: "'Helvetica Neue', Helvetica, Arial, sans-serif",
      }}
    >
      {/* Header */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 24,
          marginBottom: 20,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div
            style={{
              width: 36,
              height: 36,
              borderRadius: 8,
              background: ACCENT,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <div style={{ width: 14, height: 14, borderRadius: '50%', background: t.bg }} />
          </div>
          <div style={{ fontFamily: SPACE_GROTESK, fontWeight: 700, fontSize: 20, letterSpacing: '-0.01em' }}>
            Weather Fusion
          </div>
          <div
            style={{
              fontSize: 13,
              color: t.text3,
              borderLeft: `1px solid ${t.border}`,
              paddingLeft: 12,
              marginLeft: 2,
            }}
          >
            Weather Intelligence Platform
          </div>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              background: t.card,
              border: `1px solid ${t.border}`,
              borderRadius: 10,
              padding: '9px 14px',
              fontSize: 14,
              color: t.text2,
            }}
          >
            <div style={{ width: 7, height: 7, borderRadius: '50%', background: ACCENT }} />
            {city}
          </div>
          <div style={{ display: 'flex', background: t.trackBg, borderRadius: 9, padding: 3, gap: 2 }}>
            <button onClick={() => setUnits('C')} style={toggleButtonStyle(units === 'C', activeBg, activeColor, inactiveColor)}>
              °C
            </button>
            <button onClick={() => setUnits('F')} style={toggleButtonStyle(units === 'F', activeBg, activeColor, inactiveColor)}>
              °F
            </button>
          </div>
          <div style={{ display: 'flex', background: t.trackBg, borderRadius: 9, padding: 3, gap: 2 }}>
            <button onClick={() => setTheme('light')} style={toggleButtonStyle(!dark, activeBg, activeColor, inactiveColor)}>
              Light
            </button>
            <button onClick={() => setTheme('dark')} style={toggleButtonStyle(dark, activeBg, activeColor, inactiveColor)}>
              Dark
            </button>
          </div>
        </div>
      </div>

      {/* Provider status strip */}
      <div
        style={{
          background: t.card,
          border: `1px solid ${t.border}`,
          borderRadius: 12,
          padding: '12px 18px',
          marginBottom: 20,
          display: 'flex',
          alignItems: 'center',
          gap: 22,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ fontSize: 12, color: t.text3, textTransform: 'uppercase', letterSpacing: '0.06em', whiteSpace: 'nowrap' }}>
          {gatheringLabel}
        </div>
        {providerStatuses.map((ps) => (
          <div key={ps.name} style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
            <div style={{ width: 7, height: 7, borderRadius: '50%', background: ps.dotColor, animation: ps.animation }} />
            <span style={{ fontSize: 13, color: t.text2, fontWeight: 600 }}>{ps.name}</span>
            <span style={{ fontSize: 12, color: ps.labelColor }}>{ps.label}</span>
          </div>
        ))}
      </div>

      {/* Hero: Consensus + AI Interpretation */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.15fr', gap: 20, marginBottom: 20 }}>
        <div
          style={{
            background: t.card,
            border: `1px solid ${t.border}`,
            borderRadius: 18,
            padding: '36px 38px',
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
          }}
        >
          <div style={{ fontSize: 13, color: t.text3, textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 10 }}>
            Consensus forecast
          </div>
          <div style={{ fontFamily: SPACE_GROTESK, fontSize: 92, fontWeight: 600, lineHeight: 1, letterSpacing: '-0.03em' }}>
            {consensusTempDisplay}
          </div>
          <div style={{ fontSize: 16, color: t.text3, marginTop: 10 }}>
            Feels like {consensusFeelsDisplay} · {providers.length} provider{providers.length === 1 ? '' : 's'} averaged
          </div>
          <div style={{ display: 'flex', gap: 12, marginTop: 22 }}>
            <div style={{ flex: 1, background: tempConfidence.soft, borderRadius: 12, padding: '14px 16px' }}>
              <div style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '0.05em', color: tempConfidence.strong, marginBottom: 4 }}>
                Temp confidence
              </div>
              <div style={{ fontFamily: SPACE_GROTESK, fontSize: 20, fontWeight: 700, color: tempConfidence.strong }}>
                {tempConfidence.label}
              </div>
              <div style={{ fontSize: 12, color: tempConfidence.strong, marginTop: 2 }}>±{tempSpread}° spread</div>
            </div>
            <div style={{ flex: 1, background: rainConfidence.soft, borderRadius: 12, padding: '14px 16px' }}>
              <div style={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '0.05em', color: rainConfidence.strong, marginBottom: 4 }}>
                Rain confidence
              </div>
              <div style={{ fontFamily: SPACE_GROTESK, fontSize: 20, fontWeight: 700, color: rainConfidence.strong }}>
                {rainConfidence.label}
              </div>
              <div style={{ fontSize: 12, color: rainConfidence.strong, marginTop: 2 }}>{rainRangeLabel}</div>
            </div>
          </div>
        </div>

        <div
          style={{
            background: t.card,
            border: `1px solid ${t.border}`,
            borderRadius: 18,
            padding: '32px 36px',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 9, marginBottom: 16 }}>
            <div style={{ width: 9, height: 9, borderRadius: '50%', background: ACCENT }} />
            <div style={{ fontFamily: SPACE_GROTESK, fontSize: 19, fontWeight: 600 }}>AI Interpretation</div>
          </div>
          <div style={{ fontSize: 19, lineHeight: 1.6, color: t.text2, flex: 1 }}>{llmSummary}</div>
          <div style={{ fontSize: 12, color: t.text3, marginTop: 18, paddingTop: 16, borderTop: `1px solid ${t.borderSoft}` }}>
            Generated from provider consensus data — not an independent forecast.
          </div>
        </div>
      </div>

      {/* Rain + hourly/daily outlook — demo data, no forecast endpoint wired up yet */}
      <div style={{ background: t.card, border: `1px solid ${t.border}`, borderRadius: 18, padding: '28px 32px', marginBottom: 20 }}>
        <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', marginBottom: 6 }}>
          <div style={{ fontFamily: SPACE_GROTESK, fontSize: 18, fontWeight: 600 }}>Rain & hourly outlook (demo)</div>
          <div style={{ fontSize: 12, color: rainConfidence.strong }}>{rainDisagreementNote}</div>
        </div>
        <div style={{ fontSize: 12, color: t.text3, marginBottom: 20 }}>
          Illustrative — forecast API is not connected yet, values below are not real
        </div>
        <div style={{ display: 'flex', alignItems: 'flex-end', gap: 16, height: 220, padding: '0 4px', marginBottom: 24 }}>
          {hourly.map((h) => (
            <div
              key={h.hourLabel}
              style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', height: '100%', justifyContent: 'flex-end' }}
            >
              <div style={{ fontSize: 12, color: t.text2, fontWeight: 600, marginBottom: 8 }}>{h.highDisplay}</div>
              <div
                style={{
                  width: 22,
                  background: t.trackBg,
                  borderRadius: 11,
                  position: 'relative',
                  height: h.barHeight,
                  display: 'flex',
                  alignItems: 'flex-end',
                  overflow: 'hidden',
                }}
              >
                <div style={{ width: '100%', background: h.rainColor, borderRadius: 11, height: `${h.fillPct}%` }} />
              </div>
              <div style={{ fontSize: 12, color: t.text3, marginTop: 8 }}>{h.lowDisplay}</div>
              <div style={{ fontSize: 12, fontWeight: 700, color: h.rainColor, marginTop: 6 }}>{h.rain}%</div>
              <div style={{ fontSize: 12, color: t.text2, marginTop: 4 }}>{h.hourLabel}</div>
            </div>
          ))}
        </div>
        <div style={{ borderTop: `1px solid ${t.borderSoft}`, paddingTop: 20 }}>
          <div style={{ fontSize: 13, color: t.text3, textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 14 }}>
            5-day outlook (demo)
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: 14 }}>
            {daily.map((d) => (
              <div key={d.dayLabel} style={{ border: `1px solid ${t.border}`, borderRadius: 12, padding: 14, textAlign: 'center' }}>
                <div style={{ fontSize: 13, fontWeight: 600, color: t.text2, marginBottom: 8 }}>{d.dayLabel}</div>
                <div style={{ fontFamily: SPACE_GROTESK, fontSize: 20, fontWeight: 600 }}>{d.highDisplay}</div>
                <div style={{ fontSize: 13, color: t.text3, marginBottom: 8 }}>{d.lowDisplay}</div>
                <div style={{ fontSize: 12, fontWeight: 700, color: d.rainColor }}>{d.rain}% rain</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Rain probability by provider — real, from precip_prob (currently 0
          for both providers: neither's FetchCurrent populates real rain
          chance, that's a forecast-only field) */}
      <div style={{ background: t.card, border: `1px solid ${t.border}`, borderRadius: 16, padding: '22px 26px', marginBottom: 20 }}>
        <div style={{ fontFamily: SPACE_GROTESK, fontSize: 15, fontWeight: 600, marginBottom: 14 }}>
          Rain probability — by provider
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: `repeat(${Math.max(providers.length, 1)}, 1fr)`, gap: 20 }}>
          {providers.map((p) => (
            <div key={p.name}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, marginBottom: 5 }}>
                <span style={{ color: t.text3 }}>{p.name}</span>
                <span style={{ fontWeight: 700 }}>{p.rainPct}%</span>
              </div>
              <div style={{ height: 7, background: t.trackBg, borderRadius: 5, overflow: 'hidden' }}>
                <div style={{ height: '100%', background: p.color, borderRadius: 5, width: `${p.rainPct}%` }} />
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Provider comparison (real) + Historical accuracy (demo — no persistence yet) */}
      <div style={{ display: 'grid', gridTemplateColumns: '1.1fr 0.8fr', gap: 18 }}>
        <div style={{ background: t.card, border: `1px solid ${t.border}`, borderRadius: 14, padding: '18px 20px' }}>
          <div style={{ fontSize: 13, fontWeight: 600, color: t.text3, marginBottom: 10 }}>Provider comparison</div>
          {providers.map((p) => (
            <div
              key={p.name}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                padding: '6px 0',
                borderBottom: `1px solid ${t.borderSoft}`,
                fontSize: 12.5,
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 7, color: t.text2 }}>
                <div style={{ width: 6, height: 6, borderRadius: '50%', background: p.color }} />
                {p.name}
              </div>
              <div style={{ color: t.text3 }}>
                {p.tempDisplay} · {p.rainPct}% · {p.condition}
              </div>
            </div>
          ))}
        </div>

        <div style={{ background: t.card, border: `1px solid ${t.border}`, borderRadius: 14, padding: '18px 20px' }}>
          <div style={{ fontSize: 13, fontWeight: 600, color: t.text3, marginBottom: 10 }}>Track record (30d, demo)</div>
          {ACCURACY.map((a) => (
            <div
              key={a.name}
              style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '6px 0', fontSize: 12.5, color: t.text3 }}
            >
              <span>{a.name}</span>
              <span style={{ fontWeight: 700, color: 'oklch(0.55 0.13 150)' }}>{a.pct}%</span>
            </div>
          ))}
        </div>
      </div>

      <div style={{ textAlign: 'center', fontSize: 12, color: t.text3, marginTop: 32 }}>
        © {new Date().getFullYear()} Amirhosein Arabhaji. All rights reserved.
      </div>
    </div>
  );
}
