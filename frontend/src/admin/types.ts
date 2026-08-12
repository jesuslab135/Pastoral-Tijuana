// Shapes of the authenticated admin API, mirroring the Go handlers field for
// field. The public site's `lib/api.ts` describes a different, read-only view
// of the same tables (nested group, resolved color), so the two stay apart.

export type Role = 'parroco' | 'secretaria';
export type Rank = 'solemnidad' | 'fiesta' | 'memoria' | 'parroquial';

export interface AdminUser {
  id: string;
  email: string;
  display_name: string;
  role: Role;
  is_active: boolean;
}

export interface AdminEvent {
  id: string;
  title: string;
  description: string;
  place: string;
  starts_at: string;
  ends_at: string;
  group_id: string;
  rank: Rank;
  color_override: string | null;
  published_at: string | null;
  cancelled_at: string | null;
}

export interface EventInput {
  title: string;
  description: string;
  place: string;
  starts_at: string;
  ends_at: string;
  group_id: string;
  rank: Rank;
  color_override: string | null;
}

export type BroadcastState = 'queued' | 'sent' | 'failed' | 'dead';

export interface Broadcast {
  id: string;
  event_id: string;
  event_title: string;
  channel_id: string;
  channel_name: string;
  channel_kind: 'whatsapp' | 'email';
  kind: 'published' | 'updated' | 'cancelled';
  state: BroadcastState;
  attempt: number;
  last_error: string | null;
  sent_at: string | null;
  created_at: string;
  /** True for what the stub WhatsApp sender only pretended to send. */
  simulated: boolean;
}

export interface Channel {
  id: string;
  kind: 'whatsapp' | 'email';
  name: string;
  target: string;
  group_id: string | null;
  is_active: boolean;
}

export interface ChannelInput {
  kind: Channel['kind'];
  name: string;
  target: string;
  group_id: string | null;
  is_active: boolean;
}

// Email and password are read on create only; an update carries them along
// harmlessly because the handler ignores both. An empty password means the
// account signs in by magic link alone.
export interface UserInput {
  email: string;
  display_name: string;
  role: Role;
  password: string;
}

export interface Group {
  id: string;
  name: string;
  slug: string;
}

export interface ApiError {
  code: string;
  message: string;
}

/**
 * The message to show for a failed request. Every API error carries Spanish
 * copy written for the parish team, so it is shown verbatim; the fallback only
 * covers a request that never reached a handler.
 */
export function apiError(e: unknown): string {
  const body = (e as { data?: { error?: Partial<ApiError> } } | undefined)?.data;
  return body?.error?.message ?? 'Algo salió mal.';
}
