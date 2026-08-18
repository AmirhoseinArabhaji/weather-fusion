// Mirrors backend/internal/models/weather.go and the SSE payloads built in
// backend/internal/handlers/weather.go. Keep field names/casing in sync with the Go JSON tags.

export type WeatherCondition = 'clear' | 'cloudy' | 'rain' | 'snow' | 'sleet' | 'thunder' | 'fog' | 'unknown';

export interface Location {
  city: string;
  country: string;
  latitude: number;
  longitude: number;
  timezone: string;
}

export interface WeatherObservation {
  id: string;
  location: Location;
  provider: string;
  temperature: number;
  feels_like: number;
  humidity: number;
  pressure: number;
  wind_speed: number;
  wind_dir: number;
  visibility: number;
  uv_index: number;
  precip_prob: number;
  condition: WeatherCondition;
  // Not every provider reports a day/night signal — see backend comments.
  is_day?: boolean;
  description: string;
  raw_response: unknown;
  observed_at: string;
  fetched_at: string;
}

export interface ConsensusResult {
  location: Location;
  temperature: number;
  temp_std_dev: number;
  humidity: number;
  wind_speed: number;
  precip_prob: number;
  condition: WeatherCondition;
  is_day: boolean; // majority vote, defaults true when no provider reports one
  confidence: number;
  providers: WeatherObservation[];
  generated_at: string;
}

export interface ProviderEvent {
  provider: string;
  status: 'ok' | 'error';
  data?: WeatherObservation;
  error?: string;
}

// Sent separately from "consensus" — the LLM call runs after the numeric
// data is already flushed, so this arrives a beat later (or with `error`).
export interface SummaryEvent {
  summary?: string;
  error?: string;
}

export interface DailyForecast {
  date: string;
  temp_min: number;
  temp_max: number;
  // Only set by the merged consensus.MergeDaily result (0 on a single
  // provider's raw entry) — cross-provider disagreement on this day, not
  // the day's own temperature range (that's temp_min/temp_max above).
  temp_spread: number;
  humidity: number;
  wind_speed: number;
  precip_prob: number;
  condition: WeatherCondition;
  description: string;
}

export interface ProviderForecast {
  location: Location;
  provider: string;
  days: DailyForecast[];
  raw_response: unknown;
  fetched_at: string;
}

export interface HourlyForecast {
  time: string;
  temperature: number;
  precip_prob: number;
  condition: WeatherCondition;
  description: string;
}

export interface ProviderHourlyForecast {
  location: Location;
  provider: string;
  hours: HourlyForecast[];
  raw_response: unknown;
  fetched_at: string;
}

// Merged hourly consensus — temp_min/temp_max are the spread across
// providers for that hour (each provider gives one value, not a range),
// playing the same role ConsensusResult.temp_std_dev plays for current weather.
export interface ConsensusHourly {
  time: string;
  temperature: number;
  temp_min: number;
  temp_max: number;
  precip_prob: number;
  condition: WeatherCondition;
}

// Mirrors backend/internal/geocoding.LocationMatch (GET /api/v1/locations/search).
export interface LocationMatch {
  name: string;
  admin1: string;
  country: string;
  lat: number;
  lon: number;
}

export interface DailyProviderEvent {
  provider: string;
  status: 'ok' | 'error';
  data?: ProviderForecast;
  error?: string;
}

export interface HourlyProviderEvent {
  provider: string;
  status: 'ok' | 'error';
  data?: ProviderHourlyForecast;
  error?: string;
}
