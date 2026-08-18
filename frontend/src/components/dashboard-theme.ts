import type { ComponentType } from 'react';
import {
  WiDaySunny,
  WiNightClear,
  WiDayCloudy,
  WiNightAltCloudy,
  WiDayRain,
  WiNightAltRain,
  WiDaySnow,
  WiNightAltSnow,
  WiDaySleet,
  WiNightAltSleet,
  WiDayThunderstorm,
  WiNightAltThunderstorm,
  WiDayFog,
  WiNightFog,
  WiNA,
} from 'weather-icons-react';
import type { WeatherCondition } from '@/lib/weather-types';

// Assigned to real providers by arrival order — provider identity/count comes
// from the backend now, not a fixed list, so colors can't be keyed by name.
export const PROVIDER_PALETTE = [
  'oklch(0.68 0.16 235)',
  'oklch(0.72 0.15 150)',
  'oklch(0.76 0.14 75)',
  'oklch(0.68 0.15 320)',
  'oklch(0.68 0.16 20)',
];

export function formatHourLabel(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' });
}

export function formatDayLabel(iso: string): string {
  return new Date(iso).toLocaleDateString([], { weekday: 'short' });
}

export function capitalize(s: string): string {
  return s.length > 0 ? s.charAt(0).toUpperCase() + s.slice(1) : s;
}

type WeatherIcon = ComponentType<{ size?: number | string; color?: string }>;

// The plain WiNight{Cloudy,Rain,Snow,Sleet,Thunderstorm} icons draw the moon
// as an unmarked circle (indistinguishable from a sun at a glance) — the
// "Alt" variants draw an actual crescent, verified by rasterizing both and
// comparing. WiNightClear/WiNightFog don't have this problem, no Alt needed.
export const CONDITION_ICON: Record<WeatherCondition, { day: WeatherIcon; night: WeatherIcon }> = {
  clear: { day: WiDaySunny, night: WiNightClear },
  cloudy: { day: WiDayCloudy, night: WiNightAltCloudy },
  rain: { day: WiDayRain, night: WiNightAltRain },
  snow: { day: WiDaySnow, night: WiNightAltSnow },
  sleet: { day: WiDaySleet, night: WiNightAltSleet },
  thunder: { day: WiDayThunderstorm, night: WiNightAltThunderstorm },
  fog: { day: WiDayFog, night: WiNightFog },
  unknown: { day: WiNA, night: WiNA },
};

export function weatherIconFor(condition: WeatherCondition, isDay: boolean): WeatherIcon {
  return isDay ? CONDITION_ICON[condition].day : CONDITION_ICON[condition].night;
}

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

export function themeTokens(dark: boolean): ThemeTokens {
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

export interface Confidence {
  label: 'High' | 'Medium' | 'Low';
  soft: string;
  ring: string;
  strong: string;
  pips: { color: string }[];
}

function pips(filled: number, color: string, dim: string): { color: string }[] {
  return Array.from({ length: 5 }, (_, i) => ({ color: i < filled ? color : dim }));
}

export function confidenceFor(std: number, lowThresh: number, highThresh: number, dark: boolean): Confidence {
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

export interface ChipMeta {
  label: string;
  dotColor: string;
  dotGlow: string;
  labelColor: string;
  chipBg: string;
  chipBorder: string;
}

export function chipMetaFor(status: 'ok' | 'error', dark: boolean): ChipMeta {
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

export const mean = (arr: number[]) => arr.reduce((a, b) => a + b, 0) / arr.length;
export const stddev = (arr: number[]) => {
  const m = mean(arr);
  return Math.sqrt(mean(arr.map((v) => (v - m) ** 2)));
};

// Same two breakpoints drive color/gradient/glow below — named once so they
// can't drift out of sync across the three lookups.
const RAIN_LIKELY = 65;
const RAIN_POSSIBLE = 35;

export const rainColorFor = (r: number, dark: boolean) =>
  r >= RAIN_LIKELY ? 'oklch(0.62 0.16 245)' : r >= RAIN_POSSIBLE ? 'oklch(0.74 0.14 75)' : dark ? 'oklch(0.6 0.04 250)' : 'oklch(0.72 0.05 240)';

export const gradFor = (r: number, dark: boolean) =>
  r >= RAIN_LIKELY
    ? 'linear-gradient(180deg, oklch(0.7 0.15 235), oklch(0.55 0.18 262))'
    : r >= RAIN_POSSIBLE
      ? 'linear-gradient(180deg, oklch(0.82 0.13 85), oklch(0.68 0.15 55))'
      : dark
        ? 'linear-gradient(180deg, oklch(0.62 0.04 250), oklch(0.48 0.03 255))'
        : 'linear-gradient(180deg, oklch(0.82 0.04 245), oklch(0.7 0.05 245))';

export const glowFor = (r: number) =>
  r >= RAIN_LIKELY ? '0 0 22px oklch(0.6 0.17 250 / 0.55)' : r >= RAIN_POSSIBLE ? '0 0 20px oklch(0.75 0.14 70 / 0.4)' : 'none';
