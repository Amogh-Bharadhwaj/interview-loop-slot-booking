import { getOffices, getRooms, getUsers, roomLabel, userLabel } from "../state.js";
import { scanBookings } from "../bookingScan.js";
import { getCurrentUser } from "../auth.js";

function fmt(iso) {
  return new Date(iso).toLocaleString(undefined, {
    weekday: "short", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
  });
}

export async function renderUpcoming(container) {
  container.innerHTML = `<div class="muted">Scanning your rooms for upcoming meetings…</div>`;

  const me = getCurrentUser();
  const [offices, rooms, users] = await Promise.all([getOffices(), getRooms(), getUsers()]);

  const now = new Date();
  const entries = await scanBookings(rooms, {
    days: 7,
    matches: (e) =>
      e.booking_state !== "Completed" &&
      new Date(e.start_time) >= now &&
      (e.host_user_id === me.id || (e.participant_ids || []).includes(me.id)),
  });

  const roomsById = new Map(rooms.map((r) => [r.room_id, r]));

  container.innerHTML = `
    <div class="panel">
      <h2>My upcoming meetings (next 7 days)</h2>
      <div class="muted" style="margin-bottom:10px">Includes meetings you host and ones you're invited to.</div>
      <div class="list" id="upcoming-list">
        ${entries.length ? "" : `<div class="muted">Nothing coming up.</div>`}
      </div>
    </div>
  `;

  const list = container.querySelector("#upcoming-list");
  for (const e of entries) {
    const room = roomsById.get(e.room_id);
    const role = e.host_user_id === me.id ? "Host" : "Participant";
    const item = document.createElement("div");
    item.className = "list-item";
    item.innerHTML = `
      <div>
        <div><strong>${fmt(e.start_time)}</strong> — ${room ? roomLabel(room, offices) : `Room #${e.room_id}`}</div>
        <div class="muted">${role} · Host: ${userLabel(users, e.host_user_id)}</div>
      </div>
      <span class="badge ${e.booking_state}">${e.booking_state}</span>
    `;
    list.appendChild(item);
  }
}
