# Pending tasks after the production deploy (2026-08-12)

Site is LIVE: https://pastoral.jesuslab135.com (Plan 6 merged as `d288e87`). These are the
open items, ordered by priority. Details in `docs/superpowers/plans/2026-08-10-roadmap.md`
§Plan 6 progress.

## Needs the owner

- [ ] **SMTP credentials** — `/opt/pastoral/.env` has empty `SMTP_*`, so magic links and
      difusión emails are only logged, never sent. Recommended: Gmail app password
      (Google account → 2FA → App passwords; `SMTP_HOST=smtp.gmail.com`, `SMTP_PORT=587`,
      `SMTP_FROM=` the address). Then on the VPS: `docker compose up -d api worker` and
      test the magic-link round trip to the párroco email.
- [ ] **Real mass schedule** — the "Horarios ordinarios de misa" card
      (`frontend/src/lib/config.ts`, `HORARIOS`) still shows mockup times
      (Do 7/9/12/19 · L–V 7 · Sá 7/19). Confirm with the párroco, edit, push to `main`
      (auto-deploys).
- [ ] **Phone webcal test** — subscribe `webcal://pastoral.jesuslab135.com/calendario.ics`
      on a phone; the two published events (café 13-ago, comidas 17-sep) should appear.
- [ ] **Regenerate the GitHub PAT** used during setup (it was shared in chat). Only the
      VPS `docker login ghcr.io` needs one (`read:packages` scope); after rotating, log in
      again as `deploy` on the VPS.
- [ ] **Confirm the rotated VPS passwords are stored** (root + deploy sudo) — they exist
      only in the owner's password manager.
- [ ] **Create the difusión channels** — the production database deliberately has zero
      channels, so publishing an event currently notifies nobody. The párroco creates the
      real WhatsApp/email channels in the Difusión screen.

## Engineering debt (biggest risk first)

- [ ] **Plan 5 review + tests** — ~3,900 lines of admin panel merged with no review and no
      tests (`docs/2026-08-12_handoff-admin-frontend.md`). Suggested: `/code-review high`
      on the admin code; cheapest tests first: `admin/dates.ts`, `eventos/form.ts`,
      `eventos/casts.ts` (pure functions). Review the event editor first (weakest
      scrutiny). Known suspects: `ListBroadcasts` INNER JOIN → LEFT JOIN;
      `deleteEvent`/`deleteChannel`/`logout` typed `void` but the API answers 204.
- [ ] **E2E Playwright smoke** (spec §11) — outstanding since Plan 4: create → publish →
      delete through the UI has never been exercised end to end.
- [ ] **Change-password UI** — none exists; the setup-generated párroco password is
      permanent until built (`PUT /admin/users/{id}` does not accept a password).
- [ ] **Reboot self-heal test** — reboot the VPS with the stack running and confirm all
      five services return (restart policies should handle it; unverified post-deploy).
- [ ] Cosmetic: the frontend ships no favicon; chi answers 405 to HEAD on the `.ics`
      feeds (GET works — pre-existing behavior).

## Standing decisions (do not undo by accident)

- **blitz is STOPPED on the VPS** (owner decision 2026-08-12): restarting it collides with
  pastoral on ports 80/443. Its volumes are intact (`docker compose -p blitz-prod start`
  would bring it back — and break pastoral).
- Deploys are `main`-only; there is no staging.
- The production database starts clean: no seed channels (migration 00005 deleted; the
  next migration number is 00006 — see the note in `backend/internal/store/migrate.go`).
- `PUBLIC_BASE_URL` is permanent once parishioners subscribe (.ics UIDs).
