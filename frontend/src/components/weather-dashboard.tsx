'use client';

import { useEffect, useState, type CSSProperties } from 'react';
import { useLocation } from '@/hooks/use-location';
import { useWeatherStream } from '@/hooks/use-weather-stream';
import { useForecastStream } from '@/hooks/use-forecast-stream';
import LocationAutocomplete from './location-autocomplete';
import {
  PROVIDER_PALETTE,
  formatHourLabel,
  formatDayLabel,
  capitalize,
  CONDITION_ICON,
  weatherIconFor,
  themeTokens,
  confidenceFor,
  chipMetaFor,
  mean,
  stddev,
  rainColorFor,
  gradFor,
  glowFor,
} from './dashboard-theme';

type Units = 'C' | 'F';
type Theme = 'light' | 'dark';

const SPACE_GROTESK = "var(--font-space-grotesk), 'Space Grotesk', sans-serif";
const MONO = "var(--font-jetbrains-mono), 'JetBrains Mono', monospace";

export default function WeatherDashboard() {
  const { location, loading: locationLoading, error: locationError, setManualLocation } = useLocation();

  const [units, setUnits] = useState<Units>('C');
  const [theme, setTheme] = useState<Theme>('light');

  // Desktop layout is untouched below — isMobile only swaps in narrower
  // values at the handful of spots that actually get cramped on a phone.
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia('(max-width: 640px)');
    const update = () => setIsMobile(mq.matches);
    update();
    mq.addEventListener('change', update);
    return () => mq.removeEventListener('change', update);
  }, []);

  // Drives the local-time clock in the consensus panel. 15s is plenty for a
  // minute-precision display without re-rendering every second for nothing.
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 15000);
    return () => clearInterval(id);
  }, []);

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
  // delta conversion: a spread (max - min) is not an absolute temp, so it
  // must scale by 9/5 without the +32 offset toValue applies.
  const toDelta = (c: number) => (units === 'F' ? Math.round((c * 9) / 5) : Math.round(c));
  const displaySpread = (c: number) => `±${toDelta(c)}°`;

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

  // Bar container height scales with the hour's actual average temperature.
  // Provider disagreement (temp_max - temp_min) is shown as a separate ±X°
  // label rather than folded into the fill height — an hour has one real
  // reading, so overloading bar/fill geometry with a second variable (spread)
  // on top of temp + rain color made none of the three easy to read at a glance.
  const hourlyRaw = (forecast.hourly ?? []).slice(0, 24);
  const hGlobalHigh = hourlyRaw.length > 0 ? Math.max(...hourlyRaw.map((h) => h.temperature)) : 0;
  const hGlobalLow = hourlyRaw.length > 0 ? Math.min(...hourlyRaw.map((h) => h.temperature)) : 0;
  const hourlyRange = hGlobalHigh - hGlobalLow || 1;

  const hourly = hourlyRaw.map((h) => {
    const rain = Math.round(h.precip_prob * 100);
    return {
      hourLabel: formatHourLabel(h.time),
      tempDisplay: displayDeg(h.temperature),
      spreadDisplay: displaySpread(h.temp_max - h.temp_min),
      rain,
      barHeight: `${Math.round(66 + ((h.temperature - hGlobalLow) / hourlyRange) * 128)}px`,
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
      icon: weatherIconFor(d.condition, true), // a whole day has no single day/night state
      rainColor: rainColorFor(rain, dark),
    };
  });

  const consensusTempDisplay = stream.consensus ? toValue(stream.consensus.temperature) : '—';
  const unitSymbol = units === 'F' ? '°F' : '°C';
  const consensusFeelsDisplay = providers.length > 0 ? display(mean(providers.map((p) => p.feelsC))) : '—';
  const consensusCondition = stream.consensus ? capitalize(stream.consensus.condition) : 'Gathering conditions…';
  const consensusIconSet = CONDITION_ICON[stream.consensus?.condition ?? 'unknown'];
  const ConsensusConditionIcon = (stream.consensus?.is_day ?? true) ? consensusIconSet.day : consensusIconSet.night;
  const localTimeDisplay = (() => {
    const timezone = stream.consensus?.location.timezone;
    if (!timezone) return null;
    try {
      return new Intl.DateTimeFormat([], { timeZone: timezone, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(now);
    } catch {
      return null; // an unrecognized timezone string shouldn't crash the render
    }
  })();
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

      <div style={{ position: 'relative', maxWidth: 1480, margin: '0 auto', padding: isMobile ? '18px 16px 40px' : '26px 40px 72px' }}>
        {/* Nav */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 20, marginBottom: 22, flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 13 }}>
            <img
              src="/icon.png"
              alt="Weather Fusion"
              width={34}
              height={34}
              style={{ width: 50, height: 50, borderRadius: '50%', boxShadow: '0 0 22px oklch(0.6 0.16 250 / 0.5)' }}
            />
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

          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: isMobile ? 'wrap' : 'nowrap', width: isMobile ? '100%' : undefined }}>
            {/* Always editable — GPS/IP resolution runs in the background (see
                useLocation) and fills this in once it lands, but typing here
                and hitting Enter overrides it immediately, whether that
                resolution is still pending, still running, or already failed. */}
            <div style={{ flex: isMobile ? '1 1 100%' : undefined }}>
              <LocationAutocomplete
                t={t}
                dark={dark}
                hasError={!!locationError}
                loading={locationLoading}
                minWidth={isMobile ? 0 : 250}
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
            </div>
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
          className={isMobile ? 'wf-hide-scrollbar' : undefined}
          style={{
            background: t.glass,
            border: `1px solid ${t.border}`,
            borderRadius: 16,
            padding: '13px 20px',
            marginBottom: 18,
            display: 'flex',
            alignItems: 'center',
            gap: 18,
            flexWrap: isMobile ? 'nowrap' : 'wrap',
            overflowX: isMobile ? 'auto' : 'hidden',
            position: 'relative',
            overflowY: 'hidden',
            // native scrollbar reserves space along the bottom edge on mobile,
            // which visually un-centers the row against the box's full height
            scrollbarWidth: isMobile ? 'none' : undefined,
            msOverflowStyle: isMobile ? 'none' : undefined,
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
          <div style={{ display: 'flex', alignItems: 'center', gap: 9, whiteSpace: 'nowrap', flexShrink: 0 }}>
            <span style={{ fontFamily: MONO, fontSize: 10.5, letterSpacing: '0.14em', textTransform: 'uppercase', color: t.text3, lineHeight: 1 }}>
              {gatheringLabel}
            </span>
            <span style={{ fontFamily: MONO, fontSize: 11, fontWeight: 600, color: t.text2, lineHeight: 1 }}>{loadedCountLabel}</span>
          </div>
          <div style={{ width: 1, height: 18, background: t.border, flexShrink: 0 }} />
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
                flexShrink: 0,
                whiteSpace: 'nowrap',
              }}
            >
              <div style={{ width: 6, height: 6, borderRadius: '50%', background: ps.dotColor, boxShadow: ps.dotGlow, flexShrink: 0 }} />
              <span style={{ fontSize: 12.5, fontWeight: 600, color: t.text2, lineHeight: 1 }}>{ps.name}</span>
              <span style={{ fontFamily: MONO, fontSize: 10.5, letterSpacing: '0.05em', color: ps.labelColor, lineHeight: 1 }}>{ps.label}</span>
            </div>
          ))}
        </div>

        {/* Hero */}
        <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : '1fr 1.2fr', gap: 18, marginBottom: 18 }}>
          <div
            style={{
              position: 'relative',
              borderRadius: 24,
              padding: isMobile ? '24px 22px' : '34px 36px',
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
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 14 }}>
                <div style={{ fontFamily: MONO, fontSize: 10.5, letterSpacing: '0.16em', textTransform: 'uppercase', color: t.heroLabel }}>
                  Consensus forecast
                </div>
                {localTimeDisplay && (
                  <div style={{ fontFamily: MONO, fontSize: 12, fontWeight: 600, color: t.heroText, opacity: 0.85 }}>{localTimeDisplay}</div>
                )}
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
                  <div style={{ fontSize: isMobile ? 60 : 104, fontWeight: 600, lineHeight: 0.9, letterSpacing: '-0.045em', color: t.heroText }}>
                    {consensusTempDisplay}
                  </div>
                  <div style={{ fontSize: isMobile ? 20 : 30, fontWeight: 500, color: t.heroLabel, marginTop: 8 }}>{unitSymbol}</div>
                </div>
                {/* centered in the space between the temp and the panel's right edge, not flush against it.
                    minWidth:0 + width:100%/maxWidth (instead of a fixed px size) lets it shrink to fit
                    whatever room is left at in-between widths instead of overflowing the panel. */}
                <div style={{ flex: 1, minWidth: 0, display: 'flex', justifyContent: 'center' }}>
                  <div
                    style={{
                      minWidth: 0,
                      width: '100%',
                      maxWidth: isMobile ? 90 : 128,
                      aspectRatio: '1',
                      borderRadius: 28,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      background: 'linear-gradient(155deg, oklch(1 0 0 / 0.18), oklch(1 0 0 / 0.04))',
                    }}
                  >
                    <ConsensusConditionIcon size="65%" color={t.heroText} />
                  </div>
                </div>
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
              padding: isMobile ? '22px 20px' : '30px 34px',
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
                ±X° = provider spread
              </div>
            </div>
          </div>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 12, height: 230, marginTop: 22, overflowX: 'auto', paddingBottom: 4 }}>
            {hourly.map((h, i) => (
              <div
                key={`${h.hourLabel}-${i}`}
                style={{
                  flex: '0 0 44px',
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  height: '100%',
                  justifyContent: 'flex-end',
                  animation: 'riseIn 0.5s ease both',
                }}
              >
                <div style={{ fontFamily: MONO, fontSize: 12, fontWeight: 600, color: t.text, marginBottom: 9 }}>{h.tempDisplay}</div>
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
                  <div style={{ width: '100%', background: h.gradient, borderRadius: 17, height: '100%', boxShadow: h.glow }} />
                </div>
                <div style={{ fontFamily: MONO, fontSize: 10.5, color: t.text3, marginTop: 9 }}>{h.spreadDisplay}</div>
                <div style={{ fontFamily: MONO, fontSize: 11.5, fontWeight: 600, color: h.rainColor, marginTop: 8 }}>{h.rain}%</div>
                <div style={{ fontSize: 12, color: t.text2, marginTop: 5 }}>{h.hourLabel}</div>
              </div>
            ))}
          </div>
        </div>

        {/* Daily */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: isMobile ? `repeat(${Math.max(daily.length, 1)}, 150px)` : `repeat(${Math.max(daily.length, 1)}, 1fr)`,
            gridAutoFlow: isMobile ? 'column' : undefined,
            overflowX: isMobile ? 'auto' : undefined,
            gap: 14,
            marginBottom: 18,
          }}
        >
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
                <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
                  <d.icon size={18} color={t.text3} />
                  <div style={{ fontSize: 13.5, fontWeight: 600, color: t.text }}>{d.dayLabel}</div>
                </div>
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
        <div style={{ display: 'grid', gridTemplateColumns: '1fr', gap: 18 }}>
          <div style={{ background: t.glass, border: `1px solid ${t.border}`, borderRadius: 22, padding: isMobile ? '20px 16px' : '26px 30px' }}>
            <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', marginBottom: 18, flexWrap: 'wrap', gap: 6 }}>
              <div style={{ fontSize: 16, fontWeight: 600, letterSpacing: '-0.01em' }}>Provider signals</div>
              <div style={{ fontFamily: MONO, fontSize: 10.5, color: rainConfidence.strong }}>{rainDisagreementNote}</div>
            </div>
            {providers.map((p) => (
              <div
                key={p.name}
                style={{
                  display: 'grid',
                  gridTemplateColumns: isMobile ? '92px 40px 1fr 36px' : '150px 62px 1fr 54px',
                  alignItems: 'center',
                  gap: isMobile ? 10 : 16,
                  padding: '11px 0',
                  borderBottom: `1px solid ${t.borderSoft}`,
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 9, minWidth: 0 }}>
                  <div style={{ width: 7, height: 7, borderRadius: '50%', background: p.color, boxShadow: `0 0 9px ${p.color}`, flexShrink: 0 }} />
                  <span style={{ fontSize: 13.5, fontWeight: 600, color: t.text2, overflow: 'hidden', textOverflow: 'ellipsis' }}>{p.name}</span>
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
        </div>

        <div style={{ textAlign: 'center', fontSize: 12, color: t.text3, marginTop: 32 }}>
          © {new Date().getFullYear()} Amirhosein Arabhaji. All rights reserved.
        </div>
      </div>
    </div>
  );
}
