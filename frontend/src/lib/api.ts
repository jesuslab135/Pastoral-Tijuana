// Typed client for the public read API. Plain fetch: the public site never
// authenticates, so there is nothing for an interceptor to do.

export type SeasonColor = 'verde' | 'violeta' | 'rosa' | 'blanco_oro' | 'rojo';
export type Rank = 'solemnidad' | 'fiesta' | 'memoria' | 'parroquial';

export interface ApiGroup {
  id: string;
  name: string;
  slug: string;
}

export interface ApiEvent {
  id: string;
  title: string;
  description: string;
  place: string;
  starts_at: string;
  ends_at: string;
  group: ApiGroup;
  rank: Rank;
  /** The season_color enum name, not a hex value. */
  color: SeasonColor;
}

export interface ApiSeason {
  name: string;
  color: SeasonColor;
  start: string;
  end: string;
}

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url, { headers: { Accept: 'application/json' } });
  if (!res.ok) {
    throw new Error(`${url} respondió ${res.status}`);
  }
  return (await res.json()) as T;
}

export async function fetchEvents(from: string, to: string): Promise<ApiEvent[]> {
  const body = await getJSON<{ events: ApiEvent[] | null }>(
    `/api/v1/events?from=${from}&to=${to}`,
  );
  return body.events ?? [];
}

export async function fetchSeasons(year: number): Promise<ApiSeason[]> {
  const body = await getJSON<{ seasons: ApiSeason[] | null }>(`/api/v1/seasons?year=${year}`);
  return body.seasons ?? [];
}

export async function fetchGroups(): Promise<ApiGroup[]> {
  const body = await getJSON<{ groups: ApiGroup[] | null }>('/api/v1/groups');
  return body.groups ?? [];
}
