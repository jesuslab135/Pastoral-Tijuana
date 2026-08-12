// The panel's whole data layer. RTK Query rides an axios instance rather than
// fetch so one interceptor can handle the expired session for every screen.

import { createApi } from '@reduxjs/toolkit/query/react';
import type { BaseQueryFn } from '@reduxjs/toolkit/query/react';
import axios from 'axios';
import type { AxiosError, Method } from 'axios';

import type {
  AdminEvent,
  AdminUser,
  ApiError,
  Broadcast,
  BroadcastState,
  Channel,
  ChannelInput,
  EventInput,
  Group,
  UserInput,
} from './types';

const http = axios.create({ baseURL: '/api/v1' });

http.interceptors.response.use(undefined, (err) => {
  const s = err.response?.status;
  // A dead session anywhere in the panel lands on the login screen — except
  // on auth calls themselves, where 401 is the screen's own business.
  if (s === 401 && !err.config.url?.startsWith('/auth') && location.pathname !== '/admin/login') {
    location.assign('/admin/login');
  }
  return Promise.reject(err);
});

const axiosBaseQuery =
  (): BaseQueryFn<
    { url: string; method?: Method; data?: unknown; params?: unknown },
    unknown,
    { status?: number; data?: { error?: ApiError } }
  > =>
  async ({ url, method = 'GET', data, params }) => {
    try {
      const res = await http.request({ url, method, data, params });
      return { data: res.data };
    } catch (e) {
      const err = e as AxiosError<{ error: ApiError }>;
      return { error: { status: err.response?.status, data: err.response?.data } };
    }
  };

// Publishing an event writes outbox rows that surface as broadcasts, so every
// event mutation refreshes the difusión log alongside the calendar.
const EVENT_TAGS: Array<'Events' | 'Broadcasts'> = ['Events', 'Broadcasts'];

export const adminApi = createApi({
  reducerPath: 'adminApi',
  baseQuery: axiosBaseQuery(),
  tagTypes: ['Me', 'Events', 'Broadcasts', 'Channels', 'Users'],
  endpoints: (build) => ({
    me: build.query<AdminUser, void>({
      query: () => ({ url: '/auth/me' }),
      transformResponse: (r: { user: AdminUser }) => r.user,
      providesTags: ['Me'],
    }),
    login: build.mutation<AdminUser, { email: string; password: string }>({
      query: (data) => ({ url: '/auth/login', method: 'POST', data }),
      transformResponse: (r: { user: AdminUser }) => r.user,
      invalidatesTags: ['Me'],
    }),
    logout: build.mutation<void, void>({
      query: () => ({ url: '/auth/logout', method: 'POST' }),
      invalidatesTags: ['Me'],
      async onQueryStarted(_arg, { dispatch, queryFulfilled }) {
        await queryFulfilled;
        // Nothing cached belongs to the next person to sign in on this browser.
        dispatch(adminApi.util.resetApiState());
      },
    }),
    magicLink: build.mutation<{ message: string }, { email: string }>({
      query: (data) => ({ url: '/auth/magic-link', method: 'POST', data }),
    }),

    groups: build.query<Group[], void>({
      query: () => ({ url: '/groups' }),
      transformResponse: (r: { groups: Group[] | null }) => r.groups ?? [],
    }),

    events: build.query<AdminEvent[], { from: string; to: string }>({
      query: (params) => ({ url: '/admin/events', params }),
      transformResponse: (r: { events: AdminEvent[] | null }) => r.events ?? [],
      providesTags: ['Events'],
    }),
    getEvent: build.query<AdminEvent, string>({
      query: (id) => ({ url: `/admin/events/${id}` }),
      transformResponse: (r: { event: AdminEvent }) => r.event,
      providesTags: ['Events'],
    }),
    createEvent: build.mutation<AdminEvent, EventInput>({
      query: (data) => ({ url: '/admin/events', method: 'POST', data }),
      transformResponse: (r: { event: AdminEvent }) => r.event,
      invalidatesTags: EVENT_TAGS,
    }),
    updateEvent: build.mutation<AdminEvent, { id: string; body: EventInput }>({
      query: ({ id, body }) => ({ url: `/admin/events/${id}`, method: 'PUT', data: body }),
      transformResponse: (r: { event: AdminEvent }) => r.event,
      invalidatesTags: EVENT_TAGS,
    }),
    publishEvent: build.mutation<AdminEvent, string>({
      query: (id) => ({ url: `/admin/events/${id}/publish`, method: 'POST' }),
      transformResponse: (r: { event: AdminEvent }) => r.event,
      invalidatesTags: EVENT_TAGS,
    }),
    unpublishEvent: build.mutation<AdminEvent, string>({
      query: (id) => ({ url: `/admin/events/${id}/unpublish`, method: 'POST' }),
      transformResponse: (r: { event: AdminEvent }) => r.event,
      invalidatesTags: EVENT_TAGS,
    }),
    deleteEvent: build.mutation<void, { id: string; notify: boolean }>({
      // The API only reads `notify=false`; sending the flag either way keeps
      // the caller's intent explicit in the request log.
      query: ({ id, notify }) => ({
        url: `/admin/events/${id}`,
        method: 'DELETE',
        params: { notify: String(notify) },
      }),
      invalidatesTags: EVENT_TAGS,
    }),

    broadcasts: build.query<Broadcast[], { state?: BroadcastState; eventId?: string }>({
      query: ({ state, eventId }) => ({
        url: '/admin/broadcasts',
        params: { state, event_id: eventId },
      }),
      transformResponse: (r: { broadcasts: Broadcast[] | null }) => r.broadcasts ?? [],
      providesTags: ['Broadcasts'],
    }),
    retryBroadcast: build.mutation<Broadcast, string>({
      query: (id) => ({ url: `/admin/broadcasts/${id}/retry`, method: 'POST' }),
      transformResponse: (r: { broadcast: Broadcast }) => r.broadcast,
      invalidatesTags: ['Broadcasts'],
    }),

    channels: build.query<Channel[], void>({
      query: () => ({ url: '/admin/channels' }),
      transformResponse: (r: { channels: Channel[] | null }) => r.channels ?? [],
      providesTags: ['Channels'],
    }),
    createChannel: build.mutation<Channel, ChannelInput>({
      query: (data) => ({ url: '/admin/channels', method: 'POST', data }),
      transformResponse: (r: { channel: Channel }) => r.channel,
      invalidatesTags: ['Channels'],
    }),
    updateChannel: build.mutation<Channel, { id: string; body: ChannelInput }>({
      query: ({ id, body }) => ({ url: `/admin/channels/${id}`, method: 'PUT', data: body }),
      transformResponse: (r: { channel: Channel }) => r.channel,
      invalidatesTags: ['Channels'],
    }),
    deleteChannel: build.mutation<void, string>({
      query: (id) => ({ url: `/admin/channels/${id}`, method: 'DELETE' }),
      invalidatesTags: ['Channels'],
    }),

    users: build.query<AdminUser[], void>({
      query: () => ({ url: '/admin/users' }),
      transformResponse: (r: { users: AdminUser[] | null }) => r.users ?? [],
      providesTags: ['Users'],
    }),
    createUser: build.mutation<AdminUser, UserInput>({
      query: (data) => ({ url: '/admin/users', method: 'POST', data }),
      transformResponse: (r: { user: AdminUser }) => r.user,
      invalidatesTags: ['Users'],
    }),
    updateUser: build.mutation<AdminUser, { id: string; body: UserInput }>({
      query: ({ id, body }) => ({ url: `/admin/users/${id}`, method: 'PUT', data: body }),
      transformResponse: (r: { user: AdminUser }) => r.user,
      invalidatesTags: ['Users'],
    }),
    setUserActive: build.mutation<AdminUser, { id: string; active: boolean }>({
      query: ({ id, active }) => ({
        url: `/admin/users/${id}/${active ? 'activate' : 'deactivate'}`,
        method: 'POST',
      }),
      transformResponse: (r: { user: AdminUser }) => r.user,
      invalidatesTags: ['Users'],
    }),
  }),
});

export const {
  useMeQuery,
  useLoginMutation,
  useLogoutMutation,
  useMagicLinkMutation,
  useGroupsQuery,
  useEventsQuery,
  useGetEventQuery,
  useCreateEventMutation,
  useUpdateEventMutation,
  usePublishEventMutation,
  useUnpublishEventMutation,
  useDeleteEventMutation,
  useBroadcastsQuery,
  useRetryBroadcastMutation,
  useChannelsQuery,
  useCreateChannelMutation,
  useUpdateChannelMutation,
  useDeleteChannelMutation,
  useUsersQuery,
  useCreateUserMutation,
  useUpdateUserMutation,
  useSetUserActiveMutation,
} = adminApi;
