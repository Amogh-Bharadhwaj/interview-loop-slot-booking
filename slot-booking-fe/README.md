# Slot Booking — Frontend

Vanilla HTML/CSS/JS UI for the slot booking backend. Fully independent of the
Go backend's folder structure — this directory is never imported by, or
written to by, the backend.

## Run it

1. Start the Go backend (default `http://localhost:8080`):
   ```
   cd ..
   go run main.go
   ```
2. Start this app's dev server (stdlib Python, no dependencies):
   ```
   python3 server.py
   ```
   Open http://localhost:5500

`server.py` serves the static files here and proxies any `/api/*` request to
the backend, so the browser only ever talks to one origin (no CORS needed).
Override with env vars if needed:
```
BACKEND_URL=http://localhost:9090 PORT=3000 python3 server.py
```

## Notes

- There's no real auth on the backend — "login" is a name+email lookup-or-create
  against the `Users` API. The password field isn't sent anywhere.
- Booking start times are entered in your browser's local time and sent as-is;
  the room's `timezone` field isn't applied to the conversion.
- "My Upcoming" / "Manage Bookings" scan `ViewSlots` across cached rooms for
  the next 7 days client-side, since the backend has no "list bookings by
  user" endpoint.
