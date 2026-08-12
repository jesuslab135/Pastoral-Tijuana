import { configureStore } from '@reduxjs/toolkit';

import { adminApi } from './api';

// The panel keeps no client state of its own yet: everything on screen is
// server data, so the API slice is the whole store.
export const store = configureStore({
  reducer: { [adminApi.reducerPath]: adminApi.reducer },
  middleware: (getDefaultMiddleware) => getDefaultMiddleware().concat(adminApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
