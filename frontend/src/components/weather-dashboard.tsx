'use client';

import { useState, type CSSProperties } from 'react';
import { useLocation } from '@/hooks/use-location';
import { useWeatherStream } from '@/hooks/use-weather-stream';
import { useForecastStream } from '@/hooks/use-forecast-stream';
import LocationAutocomplete from './location-autocomplete';

type Units = 'C' | 'F';
type Theme = 'light' | 'dark';

const SPACE_GROTESK = "var(--font-space-grotesk), 'Space Grotesk', sans-serif";
const MONO = "var(--font-jetbrains-mono), 'JetBrains Mono', monospace";

// Assigned to real providers by arrival order — provider identity/count comes
// from the backend now, not a fixed list, so colors can't be keyed by name.
const PROVIDER_PALETTE = [
  'oklch(0.68 0.16 235)',
  'oklch(0.72 0.15 150)',
  'oklch(0.76 0.14 75)',
  'oklch(0.68 0.15 320)',
  'oklch(0.68 0.16 20)',
];

function formatHourLabel(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: 'numeric', hour12: true }).replace(' ', '');
}

function formatDayLabel(iso: string): string {
  return new Date(iso).toLocaleDateString([], { weekday: 'short' });
}

function capitalize(s: string): string {
  return s.length > 0 ? s.charAt(0).toUpperCase() + s.slice(1) : s;
}

// Demo — no accuracy-tracking/persistence endpoint exists yet.
const ACCURACY: { name: string; pct: number }[] = [
  { name: 'openweather', pct: 91 },
  { name: 'open-meteo', pct: 88 },
  { name: 'visualcrossing', pct: 86 },
  { name: 'weatherapi', pct: 84 },
  { name: 'tomorrow.io', pct: 82 },
];

export interface ThemeTokens {
  bg: string;
  glass: string;
  border: string;
  borderSoft: string;
  trackBg: string;
  text: string;
  text2: string;
  text3: string;
  ambient: string;
  heroBg: string;
  heroBorder: string;
  heroShadow: string;
  heroText: string;
  heroSub: string;
  heroLabel: string;
}

function themeTokens(dark: boolean): ThemeTokens {
  return dark
    ? {
        bg: 'oklch(0.16 0.022 265)',
        glass: 'oklch(0.225 0.024 265)',
        border: 'oklch(0.34 0.028 265)',
        borderSoft: 'oklch(0.29 0.022 265)',
        trackBg: 'oklch(0.3 0.025 265)',
        text: 'oklch(0.96 0.008 265)',
        text2: 'oklch(0.82 0.015 265)',
        text3: 'oklch(0.63 0.02 265)',
        ambient:
          'radial-gradient(ellipse 60% 100% at 18% 0%, oklch(0.42 0.14 262 / 0.55), transparent 65%), radial-gradient(ellipse 55% 90% at 88% 0%, oklch(0.45 0.1 205 / 0.4), transparent 65%)',
        heroBg: 'linear-gradient(150deg, oklch(0.34 0.11 262), oklch(0.24 0.08 275) 55%, oklch(0.2 0.05 265))',
        heroBorder: 'oklch(0.5 0.1 262 / 0.55)',
        heroShadow: '0 20px 60px oklch(0.3 0.15 265 / 0.5)',
        heroText: 'oklch(0.98 0.01 265)',
        heroSub: 'oklch(0.82 0.03 265)',
        heroLabel: 'oklch(0.78 0.06 250)',
      }
    : {
        bg: 'oklch(0.975 0.008 260)',
        glass: 'oklch(1 0 0)',
        border: 'oklch(0.89 0.014 260)',
        borderSoft: 'oklch(0.94 0.01 260)',
        trackBg: 'oklch(0.92 0.014 260)',
        text: 'oklch(0.24 0.02 265)',
        text2: 'oklch(0.38 0.02 265)',
        text3: 'oklch(0.55 0.02 265)',
        ambient:
          'radial-gradient(ellipse 60% 100% at 18% 0%, oklch(0.78 0.11 258 / 0.4), transparent 65%), radial-gradient(ellipse 55% 90% at 88% 0%, oklch(0.85 0.09 200 / 0.4), transparent 65%)',
        heroBg: 'linear-gradient(150deg, oklch(0.55 0.16 258), oklch(0.42 0.16 275) 60%, oklch(0.36 0.13 280))',
        heroBorder: 'oklch(0.6 0.14 262 / 0.4)',
        heroShadow: '0 20px 50px oklch(0.5 0.14 265 / 0.28)',
        heroText: 'oklch(1 0 0)',
        heroSub: 'oklch(0.92 0.03 265)',
        heroLabel: 'oklch(0.88 0.06 250)',
      };
}

interface Confidence {
  label: 'High' | 'Medium' | 'Low';
  soft: string;
  ring: string;
  strong: string;
  pips: { color: string }[];
}

function pips(filled: number, color: string, dim: string): { color: string }[] {
  return Array.from({ length: 5 }, (_, i) => ({ color: i < filled ? color : dim }));
}

function confidenceFor(std: number, lowThresh: number, highThresh: number, dark: boolean): Confidence {
  const dimPip = dark ? 'oklch(0.4 0.02 265)' : 'oklch(0.88 0.015 265)';
  if (std < lowThresh) {
    return {
      label: 'High',
      soft: dark ? 'oklch(0.32 0.07 155 / 0.5)' : 'oklch(0.96 0.035 155)',
      ring: dark ? 'oklch(0.5 0.1 155 / 0.5)' : 'oklch(0.88 0.06 155)',
      strong: dark ? 'oklch(0.82 0.14 155)' : 'oklch(0.45 0.12 155)',
      pips: pips(5, dark ? 'oklch(0.78 0.15 150)' : 'oklch(0.55 0.13 150)', dimPip),
    };
  }
  if (std < highThresh) {
    return {
      label: 'Medium',
      soft: dark ? 'oklch(0.34 0.07 75 / 0.5)' : 'oklch(0.97 0.04 85)',
      ring: dark ? 'oklch(0.52 0.1 75 / 0.5)' : 'oklch(0.9 0.06 85)',
      strong: dark ? 'oklch(0.84 0.14 80)' : 'oklch(0.5 0.13 70)',
      pips: pips(3, dark ? 'oklch(0.8 0.15 80)' : 'oklch(0.62 0.14 70)', dimPip),
    };
  }
  return {
    label: 'Low',
    soft: dark ? 'oklch(0.34 0.08 25 / 0.5)' : 'oklch(0.97 0.035 30)',
    ring: dark ? 'oklch(0.52 0.11 25 / 0.5)' : 'oklch(0.91 0.05 30)',
    strong: dark ? 'oklch(0.8 0.15 28)' : 'oklch(0.54 0.16 28)',
    pips: pips(1, dark ? 'oklch(0.76 0.17 28)' : 'oklch(0.6 0.17 28)', dimPip),
  };
}

interface ChipMeta {
  label: string;
  dotColor: string;
  dotGlow: string;
  labelColor: string;
  chipBg: string;
  chipBorder: string;
}

function chipMetaFor(status: 'ok' | 'error', dark: boolean): ChipMeta {
  if (status === 'ok') {
    return {
      label: 'Live',
      dotColor: 'oklch(0.72 0.15 150)',
      dotGlow: '0 0 10px oklch(0.72 0.15 150)',
      labelColor: dark ? 'oklch(0.8 0.14 150)' : 'oklch(0.48 0.12 150)',
      chipBg: dark ? 'oklch(0.33 0.07 155 / 0.35)' : 'oklch(0.96 0.03 155)',
      chipBorder: dark ? 'oklch(0.48 0.09 155 / 0.45)' : 'oklch(0.88 0.05 155)',
    };
  }
  return {
    label: 'Failed',
    dotColor: 'oklch(0.62 0.19 25)',
    dotGlow: '0 0 10px oklch(0.62 0.19 25)',
    labelColor: dark ? 'oklch(0.76 0.16 25)' : 'oklch(0.52 0.17 25)',
    chipBg: dark ? 'oklch(0.33 0.08 25 / 0.35)' : 'oklch(0.96 0.03 25)',
    chipBorder: dark ? 'oklch(0.5 0.1 25 / 0.45)' : 'oklch(0.9 0.05 25)',
  };
}

const mean = (arr: number[]) => arr.reduce((a, b) => a + b, 0) / arr.length;
const stddev = (arr: number[]) => {
  const m = mean(arr);
  return Math.sqrt(mean(arr.map((v) => (v - m) ** 2)));
};

// Same two breakpoints drive color/gradient/glow below — named once so they
// can't drift out of sync across the three lookups.
const RAIN_LIKELY = 65;
const RAIN_POSSIBLE = 35;

const rainColorFor = (r: number, dark: boolean) =>
  r >= RAIN_LIKELY ? 'oklch(0.62 0.16 245)' : r >= RAIN_POSSIBLE ? 'oklch(0.74 0.14 75)' : dark ? 'oklch(0.6 0.04 250)' : 'oklch(0.72 0.05 240)';

const gradFor = (r: number, dark: boolean) =>
  r >= RAIN_LIKELY
    ? 'linear-gradient(180deg, oklch(0.7 0.15 235), oklch(0.55 0.18 262))'
    : r >= RAIN_POSSIBLE
      ? 'linear-gradient(180deg, oklch(0.82 0.13 85), oklch(0.68 0.15 55))'
      : dark
        ? 'linear-gradient(180deg, oklch(0.62 0.04 250), oklch(0.48 0.03 255))'
        : 'linear-gradient(180deg, oklch(0.82 0.04 245), oklch(0.7 0.05 245))';

const glowFor = (r: number) =>
  r >= RAIN_LIKELY ? '0 0 22px oklch(0.6 0.17 250 / 0.55)' : r >= RAIN_POSSIBLE ? '0 0 20px oklch(0.75 0.14 70 / 0.4)' : 'none';

export default function WeatherDashboard() {
  const { location, loading: locationLoading, error: locationError, setManualLocation } = useLocation();

  const [units, setUnits] = useState<Units>('C');
  const [theme, setTheme] = useState<Theme>('light');

  // Manually-entered locations have no coordinates (no client-side geocoding) —
  // send the city name instead and let the backend resolve it, same as every
  // other city-name query.
  const locationParams = location
    ? location.lat != null && location.lon != null
      ? { lat: location.lat, lon: location.lon, units: 'metric' as const }
      : { city: location.city, units: 'metric' as const }
    : null;

  const stream = useWeatherStream(locationParams);
  const forecast = useForecastStream(locationParams ? { ...locationParams, days: 5 } : null);

  const dark = theme === 'dark';
  const t = themeTokens(dark);
  const activeBg = dark ? 'oklch(0.45 0.14 258)' : 'oklch(0.98 0.01 260)';
  const activeColor = dark ? 'oklch(0.98 0.01 265)' : 'oklch(0.28 0.06 265)';
  const activeShadow = dark ? '0 0 14px oklch(0.55 0.18 258 / 0.7)' : '0 1px 4px oklch(0.5 0.05 265 / 0.2)';
  const inactiveColor = t.text3;
  const toggleBtn = (active: boolean, mono: boolean): CSSProperties => ({
    border: 'none',
    cursor: 'pointer',
    fontFamily: mono ? MONO : undefined,
    fontSize: 12,
    fontWeight: 600,
    padding: mono ? '7px 11px' : '7px 12px',
    borderRadius: 9,
    background: active ? activeBg : 'transparent',
    color: active ? activeColor : inactiveColor,
    boxShadow: active ? activeShadow : 'none',
    transition: 'all 0.18s',
  });

  const toValue = (c: number) => (units === 'F' ? Math.round((c * 9) / 5 + 32) : Math.round(c));
  const display = (c: number) => `${toValue(c)}°${units}`;
  const displayDeg = (c: number) => `${toValue(c)}°`;

  // Real providers, in arrival order — count and identity come from whatever
  // the backend actually has configured, not a fixed list.
  const rawProviders = stream.providerEvents
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
      };
    });

  const rainMean = rawProviders.length > 0 ? mean(rawProviders.map((p) => p.rainPct)) : 0;
  const outlierColor = dark ? 'oklch(0.78 0.16 28)' : 'oklch(0.54 0.16 28)';
  const providers = rawProviders.map((p) => ({
    ...p,
    tempDisplay: display(p.tempC),
    rainOutlierColor: Math.abs(p.rainPct - rainMean) > 25 ? outlierColor : t.text2,
  }));

  const providerStatuses = stream.providerEvents.map((ev) => {
    const meta = chipMetaFor(ev.status, dark);
    return { name: ev.provider, ...meta };
  });
  const allSettled = stream.status === 'done' || stream.status === 'error';
  const gatheringLabel = allSettled
    ? stream.status === 'error'
      ? 'Error loading data'
      : 'Providers synced'
    : 'Gathering providers';
  const loadedCountLabel = `${stream.providerEvents.length} responded`;
  const sweepAnimation = allSettled ? 'none' : 'sweep 2.2s linear infinite';

  const temps = providers.map((p) => p.tempC);
  const rains = providers.map((p) => p.rainPct);
  const tempStd = temps.length > 0 ? stddev(temps) : 0;
  const rainStd = rains.length > 0 ? stddev(rains) : 0;

  const tempConfidence = confidenceFor(tempStd, 0.8, 1.5, dark);
  const rainConfidence = confidenceFor(rainStd, 12, 22, dark);

  // Bar container height scales with the hour's high edge (temp_max); the
  // fill within it represents temp_max/temp_min spread as a proportion of the
  // visible range — how much providers disagree that hour, not a real
  // intra-hour temperature range (an hour only has one true average reading).
  const hourlyRaw = (forecast.hourly ?? []).slice(0, 8);
  const hGlobalHigh = hourlyRaw.length > 0 ? Math.max(...hourlyRaw.map((h) => h.temp_max)) : 0;
  const hGlobalLow = hourlyRaw.length > 0 ? Math.min(...hourlyRaw.map((h) => h.temp_min)) : 0;
  const hourlyRange = hGlobalHigh - hGlobalLow || 1;

  const hourly = hourlyRaw.map((h) => {
    const rain = Math.round(h.precip_prob * 100);
    return {
      hourLabel: formatHourLabel(h.time),
      highDisplay: displayDeg(h.temp_max),
      lowDisplay: displayDeg(h.temp_min),
      rain,
      barHeight: `${Math.round(66 + ((h.temp_max - hGlobalLow) / hourlyRange) * 128)}px`,
      fillPct: Math.round(((h.temp_max - h.temp_min) / hourlyRange) * 100) + 38,
      rainColor: rainColorFor(rain, dark),
      gradient: gradFor(rain, dark),
      glow: glowFor(rain),
    };
  });

  const dailyRaw = (forecast.daily ?? []).slice(0, 5);
  const daily = dailyRaw.map((d) => {
    const rain = Math.round(d.precip_prob * 100);
    return {
      dayLabel: formatDayLabel(d.date),
      highDisplay: displayDeg(d.temp_max),
      lowDisplay: displayDeg(d.temp_min),
      rain,
      condition: d.description || d.condition,
      rainColor: rainColorFor(rain, dark),
    };
  });

  const consensusTempDisplay = stream.consensus ? toValue(stream.consensus.temperature) : '—';
  const unitSymbol = units === 'F' ? '°F' : '°C';
  const consensusFeelsDisplay = providers.length > 0 ? display(mean(providers.map((p) => p.feelsC))) : '—';
  const consensusCondition = stream.consensus ? capitalize(stream.consensus.condition) : 'Gathering conditions…';
  const tempSpread = temps.length > 0 ? (Math.max(...temps) - Math.min(...temps)).toFixed(1) : '0.0';
  const rainRangeLabel = rains.length > 0 ? `${Math.min(...rains)}–${Math.max(...rains)}%` : '—';
  const rainDisagreementNote = rainConfidence.label === 'Low' ? 'Wide disagreement on rain' : 'Providers broadly agree';

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
        position: 'relative',
        overflow: 'hidden',
        transition: 'background 0.3s',
        fontFamily: SPACE_GROTESK,
      }}
    >
      {/* Ambient field */}
      <div style={{ position: 'absolute', top: 0, left: 0, right: 0, height: 620, pointerEvents: 'none', background: t.ambient }} />

      <div style={{ position: 'relative', maxWidth: 1480, margin: '0 auto', padding: '26px 40px 72px' }}>
        {/* Nav */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 20, marginBottom: 22, flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 13 }}>
            <div
              style={{
                width: 34,
                height: 34,
                borderRadius: 11,
                background: 'linear-gradient(145deg, oklch(0.68 0.16 235), oklch(0.5 0.19 275))',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                boxShadow: '0 0 22px oklch(0.6 0.16 250 / 0.5)',
              }}
            >
              <div style={{ width: 11, height: 11, borderRadius: '50%', background: '#fff', boxShadow: '0 0 10px #fff' }} />
            </div>
            <div style={{ fontWeight: 700, fontSize: 19, letterSpacing: '-0.02em' }}>Weather Fusion</div>
            <div
              style={{
                fontFamily: MONO,
                fontSize: 10.5,
                letterSpacing: '0.14em',
                textTransform: 'uppercase',
                color: t.text3,
                border: `1px solid ${t.border}`,
                borderRadius: 20,
                padding: '4px 11px',
              }}
            >
              Weather Intelligence
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            {/* Always editable — GPS/IP resolution runs in the background (see
                useLocation) and fills this in once it lands, but typing here
                and hitting Enter overrides it immediately, whether that
                resolution is still pending, still running, or already failed. */}
            <LocationAutocomplete
              t={t}
              dark={dark}
              hasError={!!locationError}
              loading={locationLoading}
              placeholder={
                locationError
                  ? 'Location lookup failed — enter a city'
                  : location
                    ? location.city || 'Current location — or enter a city'
                    : 'Resolving location… or enter a city'
              }
              onSelect={(match) => setManualLocation(match.name, { lat: match.lat, lon: match.lon })}
              onSubmitFreeText={(text) => setManualLocation(text)}
            />
            <div style={{ display: 'flex', background: t.glass, border: `1px solid ${t.border}`, borderRadius: 12, padding: 3, gap: 2 }}>
              <button onClick={() => setUnits('C')} style={toggleBtn(units === 'C', true)}>
                °C
              </button>
              <button onClick={() => setUnits('F')} style={toggleBtn(units === 'F', true)}>
                °F
              </button>
            </div>
            <div style={{ display: 'flex', background: t.glass, border: `1px solid ${t.border}`, borderRadius: 12, padding: 3, gap: 2 }}>
              <button onClick={() => setTheme('light')} style={toggleBtn(!dark, false)}>
                Light
              </button>
              <button onClick={() => setTheme('dark')} style={toggleBtn(dark, false)}>
                Dark
              </button>
            </div>
          </div>
        </div>

        {/* Provider stream rail */}
        <div
          style={{
            background: t.glass,
            border: `1px solid ${t.border}`,
            borderRadius: 16,
            padding: '13px 20px',
            marginBottom: 18,
            display: 'flex',
            alignItems: 'center',
            gap: 18,
            flexWrap: 'wrap',
            position: 'relative',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              height: 1,
              width: '34%',
              background: 'linear-gradient(90deg, transparent, oklch(0.68 0.16 235), transparent)',
              animation: sweepAnimation,
            }}
          />
          <div style={{ display: 'flex', alignItems: 'center', gap: 9, whiteSpace: 'nowrap' }}>
            <span style={{ fontFamily: MONO, fontSize: 10.5, letterSpacing: '0.14em', textTransform: 'uppercase', color: t.text3 }}>
              {gatheringLabel}
            </span>
            <span style={{ fontFamily: MONO, fontSize: 11, fontWeight: 600, color: t.text2 }}>{loadedCountLabel}</span>
          </div>
          <div style={{ width: 1, height: 18, background: t.border }} />
          {providerStatuses.map((ps) => (
            <div
              key={ps.name}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                background: ps.chipBg,
                border: `1px solid ${ps.chipBorder}`,
                borderRadius: 20,
                padding: '5px 12px 5px 10px',
                transition: 'all 0.3s',
              }}
            >
              <div style={{ width: 6, height: 6, borderRadius: '50%', background: ps.dotColor, boxShadow: ps.dotGlow }} />
              <span style={{ fontSize: 12.5, fontWeight: 600, color: t.text2 }}>{ps.name}</span>
              <span style={{ fontFamily: MONO, fontSize: 10.5, letterSpacing: '0.05em', color: ps.labelColor }}>{ps.label}</span>
            </div>
          ))}
        </div>

        {/* Hero */}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1.2fr', gap: 18, marginBottom: 18 }}>
          <div
            style={{
              position: 'relative',
              borderRadius: 24,
              padding: '34px 36px',
              overflow: 'hidden',
              background: t.heroBg,
              border: `1px solid ${t.heroBorder}`,
              boxShadow: t.heroShadow,
            }}
          >
            <div
              style={{
                position: 'absolute',
                right: -70,
                top: -70,
                width: 280,
                height: 280,
                borderRadius: '50%',
                background: 'radial-gradient(circle, oklch(0.7 0.16 235 / 0.3), transparent 68%)',
              }}
            />
            <div style={{ position: 'relative' }}>
              <div style={{ fontFamily: MONO, fontSize: 10.5, letterSpacing: '0.16em', textTransform: 'uppercase', color: t.heroLabel, marginBottom: 14 }}>
                Consensus forecast
              </div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                <div style={{ fontSize: 104, fontWeight: 600, lineHeight: 0.9, letterSpacing: '-0.045em', color: t.heroText }}>
                  {consensusTempDisplay}
                </div>
                <div style={{ fontSize: 30, fontWeight: 500, color: t.heroLabel, marginTop: 8 }}>{unitSymbol}</div>
              </div>
              <div style={{ fontSize: 15, color: t.heroSub, marginTop: 14 }}>
                {consensusCondition} · feels like {consensusFeelsDisplay}
              </div>

              <div style={{ display: 'flex', gap: 11, marginTop: 26 }}>
                <div style={{ flex: 1, background: tempConfidence.soft, border: `1px solid ${tempConfidence.ring}`, borderRadius: 15, padding: '14px 16px' }}>
                  <div style={{ fontFamily: MONO, fontSize: 9.5, letterSpacing: '0.12em', textTransform: 'uppercase', color: tempConfidence.strong, marginBottom: 8 }}>
                    Temperature
                  </div>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 7 }}>
                    <div style={{ fontSize: 21, fontWeight: 700, color: tempConfidence.strong }}>{tempConfidence.label}</div>
                    <div style={{ fontFamily: MONO, fontSize: 11, color: tempConfidence.strong, opacity: 0.75 }}>±{tempSpread}°</div>
                  </div>
                  <div style={{ display: 'flex', gap: 3, marginTop: 9 }}>
                    {tempConfidence.pips.map((pip, i) => (
                      <div key={i} style={{ flex: 1, height: 3, borderRadius: 2, background: pip.color }} />
                    ))}
                  </div>
                </div>
                <div style={{ flex: 1, background: rainConfidence.soft, border: `1px solid ${rainConfidence.ring}`, borderRadius: 15, padding: '14px 16px' }}>
                  <div style={{ fontFamily: MONO, fontSize: 9.5, letterSpacing: '0.12em', textTransform: 'uppercase', color: rainConfidence.strong, marginBottom: 8 }}>
                    Precipitation
                  </div>
                  <div style={{ display: 'flex', alignItems: 'baseline', gap: 7 }}>
                    <div style={{ fontSize: 21, fontWeight: 700, color: rainConfidence.strong }}>{rainConfidence.label}</div>
                    <div style={{ fontFamily: MONO, fontSize: 11, color: rainConfidence.strong, opacity: 0.75 }}>{rainRangeLabel}</div>
                  </div>
                  <div style={{ display: 'flex', gap: 3, marginTop: 9 }}>
                    {rainConfidence.pips.map((pip, i) => (
                      <div key={i} style={{ flex: 1, height: 3, borderRadius: 2, background: pip.color }} />
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div
            style={{
              background: t.glass,
              border: `1px solid ${t.border}`,
              borderRadius: 24,
              padding: '30px 34px',
              display: 'flex',
              flexDirection: 'column',
              position: 'relative',
              overflow: 'hidden',
            }}
          >
            <div
              style={{
                position: 'absolute',
                left: 0,
                top: 26,
                bottom: 26,
                width: 2,
                background: 'linear-gradient(180deg, transparent, oklch(0.68 0.16 235), transparent)',
              }}
            />
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div
                  style={{
                    width: 22,
                    height: 22,
                    borderRadius: 7,
                    background: 'linear-gradient(145deg, oklch(0.68 0.16 235), oklch(0.5 0.19 275))',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    boxShadow: '0 0 14px oklch(0.6 0.16 250 / 0.45)',
                  }}
                >
                  <div style={{ width: 6, height: 6, borderRadius: '50%', background: '#fff' }} />
                </div>
                <div style={{ fontSize: 18, fontWeight: 600, letterSpacing: '-0.01em' }}>AI Interpretation</div>
              </div>
              <div
                style={{
                  fontFamily: MONO,
                  fontSize: 10,
                  letterSpacing: '0.1em',
                  textTransform: 'uppercase',
                  color: t.text3,
                  border: `1px solid ${t.border}`,
                  borderRadius: 20,
                  padding: '4px 10px',
                }}
              >
                Interpreter, not forecaster
              </div>
            </div>
            <div style={{ fontSize: 19.5, lineHeight: 1.65, color: t.text2, flex: 1, letterSpacing: '-0.005em' }}>{llmSummary}</div>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 10,
                fontFamily: MONO,
                fontSize: 10.5,
                color: t.text3,
                marginTop: 20,
                paddingTop: 16,
                borderTop: `1px solid ${t.borderSoft}`,
              }}
            >
              <div style={{ width: 5, height: 5, borderRadius: '50%', background: 'oklch(0.7 0.15 150)', boxShadow: '0 0 8px oklch(0.7 0.15 150)' }} />
              Derived from provider consensus — no independent prediction is made.
            </div>
          </div>
        </div>

        {/* Hourly */}
        <div style={{ background: t.glass, border: `1px solid ${t.border}`, borderRadius: 24, padding: '28px 32px', marginBottom: 18 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 4, flexWrap: 'wrap', gap: 12 }}>
            <div style={{ fontSize: 17, fontWeight: 600, letterSpacing: '-0.01em' }}>Hourly outlook</div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontFamily: MONO, fontSize: 10.5, color: t.text3 }}>
                <div style={{ width: 9, height: 9, borderRadius: 3, background: 'oklch(0.72 0.05 240)' }} /> Dry
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontFamily: MONO, fontSize: 10.5, color: t.text3 }}>
                <div style={{ width: 9, height: 9, borderRadius: 3, background: 'oklch(0.74 0.14 75)' }} /> Possible
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontFamily: MONO, fontSize: 10.5, color: t.text3 }}>
                <div style={{ width: 9, height: 9, borderRadius: 3, background: 'oklch(0.62 0.16 245)' }} /> Likely
              </div>
              <div style={{ fontFamily: MONO, fontSize: 10.5, color: t.text3, borderLeft: `1px solid ${t.border}`, paddingLeft: 16 }}>
                Band = provider spread
              </div>
            </div>
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 12, height: 230, marginTop: 22 }}>
            {hourly.map((h, i) => (
              <div
                key={`${h.hourLabel}-${i}`}
                style={{
                  flex: 1,
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  height: '100%',
                  justifyContent: 'flex-end',
                  animation: 'riseIn 0.5s ease both',
                }}
              >
                <div style={{ fontFamily: MONO, fontSize: 12, fontWeight: 600, color: t.text, marginBottom: 9 }}>{h.highDisplay}</div>
                <div
                  style={{
                    width: '100%',
                    maxWidth: 34,
                    background: t.trackBg,
                    borderRadius: 17,
                    height: h.barHeight,
                    display: 'flex',
                    alignItems: 'flex-end',
                    overflow: 'hidden',
                    position: 'relative',
                  }}
                >
                  <div style={{ width: '100%', background: h.gradient, borderRadius: 17, height: `${h.fillPct}%`, boxShadow: h.glow }} />
                </div>
                <div style={{ fontFamily: MONO, fontSize: 11, color: t.text3, marginTop: 9 }}>{h.lowDisplay}</div>
                <div style={{ fontFamily: MONO, fontSize: 11.5, fontWeight: 600, color: h.rainColor, marginTop: 8 }}>{h.rain}%</div>
                <div style={{ fontSize: 12, color: t.text2, marginTop: 5 }}>{h.hourLabel}</div>
              </div>
            ))}
          </div>
        </div>

        {/* Daily */}
        <div style={{ display: 'grid', gridTemplateColumns: `repeat(${Math.max(daily.length, 1)}, 1fr)`, gap: 14, marginBottom: 18 }}>
          {daily.map((d, i) => (
            <div
              key={`${d.dayLabel}-${i}`}
              style={{
                background: t.glass,
                border: `1px solid ${t.border}`,
                borderRadius: 18,
                padding: 20,
                position: 'relative',
                overflow: 'hidden',
                transition: 'transform 0.2s, border-color 0.2s',
              }}
            >
              <div style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: 3, background: d.rainColor, opacity: 0.85 }} />
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 14 }}>
                <div style={{ fontSize: 13.5, fontWeight: 600, color: t.text }}>{d.dayLabel}</div>
                <div style={{ fontFamily: MONO, fontSize: 10.5, fontWeight: 600, color: d.rainColor }}>{d.rain}%</div>
              </div>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 9 }}>
                <div style={{ fontSize: 27, fontWeight: 600, letterSpacing: '-0.02em' }}>{d.highDisplay}</div>
                <div style={{ fontFamily: MONO, fontSize: 14, color: t.text3 }}>{d.lowDisplay}</div>
              </div>
              <div style={{ fontSize: 12.5, color: t.text3, marginTop: 10 }}>{d.condition}</div>
              <div style={{ height: 4, background: t.trackBg, borderRadius: 3, marginTop: 12, overflow: 'hidden' }}>
                <div style={{ height: '100%', background: d.rainColor, borderRadius: 3, width: `${d.rain}%` }} />
              </div>
            </div>
          ))}
        </div>

        {/* Provider detail */}
        <div style={{ display: 'grid', gridTemplateColumns: '1.55fr 1fr', gap: 18 }}>
          <div style={{ background: t.glass, border: `1px solid ${t.border}`, borderRadius: 22, padding: '26px 30px' }}>
            <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', marginBottom: 18 }}>
              <div style={{ fontSize: 16, fontWeight: 600, letterSpacing: '-0.01em' }}>Provider signals</div>
              <div style={{ fontFamily: MONO, fontSize: 10.5, color: rainConfidence.strong }}>{rainDisagreementNote}</div>
            </div>
            {providers.map((p) => (
              <div
                key={p.name}
                style={{
                  display: 'grid',
                  gridTemplateColumns: '150px 62px 1fr 54px',
                  alignItems: 'center',
                  gap: 16,
                  padding: '11px 0',
                  borderBottom: `1px solid ${t.borderSoft}`,
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 9 }}>
                  <div style={{ width: 7, height: 7, borderRadius: '50%', background: p.color, boxShadow: `0 0 9px ${p.color}` }} />
                  <span style={{ fontSize: 13.5, fontWeight: 600, color: t.text2 }}>{p.name}</span>
                </div>
                <div style={{ fontFamily: MONO, fontSize: 13, fontWeight: 600, color: t.text }}>{p.tempDisplay}</div>
                <div style={{ height: 6, background: t.trackBg, borderRadius: 4, overflow: 'hidden' }}>
                  <div style={{ height: '100%', background: p.color, borderRadius: 4, width: `${p.rainPct}%`, boxShadow: `0 0 10px ${p.color}` }} />
                </div>
                <div style={{ fontFamily: MONO, fontSize: 12.5, fontWeight: 600, textAlign: 'right', color: p.rainOutlierColor }}>{p.rainPct}%</div>
              </div>
            ))}
            <div style={{ fontFamily: MONO, fontSize: 10.5, color: t.text3, marginTop: 14, letterSpacing: '0.05em' }}>
              Bar = rain probability reported by that provider
            </div>
          </div>

          <div style={{ background: t.glass, border: `1px solid ${t.border}`, borderRadius: 22, padding: '26px 30px' }}>
            <div style={{ fontSize: 16, fontWeight: 600, letterSpacing: '-0.01em', marginBottom: 4 }}>Track record</div>
            <div style={{ fontFamily: MONO, fontSize: 10.5, color: t.text3, marginBottom: 18, letterSpacing: '0.05em' }}>
              Hit rate over the last 30 days — demo, no accuracy tracking yet
            </div>
            {ACCURACY.map((a) => (
              <div key={a.name} style={{ marginBottom: 14 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 6 }}>
                  <span style={{ fontSize: 13, color: t.text2 }}>{a.name}</span>
                  <span style={{ fontFamily: MONO, fontSize: 13, fontWeight: 600, color: 'oklch(0.72 0.14 150)' }}>{a.pct}%</span>
                </div>
                <div style={{ height: 6, background: t.trackBg, borderRadius: 4, overflow: 'hidden' }}>
                  <div
                    style={{
                      height: '100%',
                      background: 'linear-gradient(90deg, oklch(0.7 0.13 175), oklch(0.72 0.15 150))',
                      borderRadius: 4,
                      width: `${a.pct}%`,
                      boxShadow: '0 0 10px oklch(0.72 0.15 150 / 0.6)',
                    }}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>

        <div style={{ textAlign: 'center', fontSize: 12, color: t.text3, marginTop: 32 }}>
          © {new Date().getFullYear()} Amirhosein Arabhaji. All rights reserved.
        </div>
      </div>
    </div>
  );
}
