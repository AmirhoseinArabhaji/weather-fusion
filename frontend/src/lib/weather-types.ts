// Mirrors backend/internal/models/weather.go and the SSE payloads built in
// backend/internal/handlers/weather.go. Keep field names/casing in sync with the Go JSON tags.

export type WeatherCondition = 'clear' | 'cloudy' | 'rain' | 'snow' | 'thunder' | 'fog' | 'unknown';

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
  confidence: number;
  providers: WeatherObservation[];
  summary: string;
  generated_at: string;
}

export interface ProviderEvent {
  provider: string;
  status: 'ok' | 'error';
  data?: WeatherObservation;
  error?: string;
}
