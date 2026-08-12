/** @jsxImportSource react */

// Root of the admin island: store, router and the session gate. Astro serves a
// static page per top-level route (`pages/admin/[...all].astro`); everything
// below that is this router's business.

import type { ReactNode } from 'react';
import { Provider } from 'react-redux';
import { BrowserRouter, Navigate, Outlet, Route, Routes } from 'react-router-dom';

import Equipo from './Equipo';
import Login from './Login';
import Shell, { useSession } from './Shell';
import { useMeQuery } from './api';
import Difusion from './difusion/Difusion';
import EventoEditor from './eventos/EventoEditor';
import EventosList from './eventos/EventosList';
import { store } from './store';

// `/auth/me` is the one 401 the axios interceptor lets through, precisely so
// the gate below can answer it with the login screen instead of a page load.
function RequireSession() {
  const { data: me, isLoading, isError } = useMeQuery();
  if (isLoading) return null;
  if (isError || !me) return <Navigate to="/login" replace />;
  return <Outlet context={me} />;
}

function RequireParroco({ children }: { children: ReactNode }) {
  const me = useSession();
  if (me.role !== 'parroco') return <Navigate to="/eventos" replace />;
  return <>{children}</>;
}

export default function App() {
  return (
    <Provider store={store}>
      <BrowserRouter basename="/admin">
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route element={<RequireSession />}>
            <Route element={<Shell />}>
              <Route index element={<Navigate to="/eventos" replace />} />
              <Route path="/eventos" element={<EventosList />} />
              <Route path="/eventos/nuevo" element={<EventoEditor />} />
              <Route path="/eventos/:id" element={<EventoEditor />} />
              <Route path="/difusion" element={<Difusion />} />
              <Route
                path="/equipo"
                element={
                  <RequireParroco>
                    <Equipo />
                  </RequireParroco>
                }
              />
              <Route path="*" element={<Navigate to="/eventos" replace />} />
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
    </Provider>
  );
}
