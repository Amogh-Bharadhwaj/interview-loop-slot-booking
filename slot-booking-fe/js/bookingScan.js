import { api } from "./api.js";

function dateStr(d) {
  return d.toISOString().slice(0, 10);
}

// There is no "list bookings by user" endpoint on the backend, so this scans
// ViewSlots across every cached room for a bounded day range (default: today
// .. +6) and returns the flattened, matching timeline entries. This is an
// N = rooms * days set of requests — fine at local-dev scale, but a real
// "my bookings" index would be a backend follow-up rather than a client scan.
export async function scanBookings(rooms, { days = 7, matches } = {}) {
  const today = new Date();
  today.setHours(0, 0, 0, 0);

  const results = [];
  for (const room of rooms) {
    for (let i = 0; i < days; i++) {
      const d = new Date(today);
      d.setDate(d.getDate() + i);
      const date = dateStr(d);

      let data;
      try {
        data = await api.viewSlots(room.room_id, date);
      } catch {
        continue; // room may not exist yet / transient error — skip this slice
      }

      for (const entry of data.timeline) {
        if (entry.type !== "booking") continue;
        if (!matches(entry)) continue;
        results.push({ ...entry, room_id: room.room_id, date });
      }
    }
  }

  results.sort((a, b) => new Date(a.start_time) - new Date(b.start_time));
  return results;
}
