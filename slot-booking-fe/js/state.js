import { api } from "./api.js";

// Tiny in-memory cache shared across tabs. Offices/rooms/users change rarely
// within a session, so we fetch once and let mutating actions (Setup tab)
// invalidate explicitly rather than refetching on every tab switch.
const state = {
  offices: null,
  rooms: null,
  users: null,
};

export async function getOffices(force = false) {
  if (force || !state.offices) state.offices = await api.listOffices();
  return state.offices;
}

export async function getRooms(force = false) {
  if (force || !state.rooms) state.rooms = await api.listRooms();
  return state.rooms;
}

export async function getUsers(force = false) {
  if (force || !state.users) state.users = await api.listUsers();
  return state.users;
}

export function invalidateOfficesAndRooms() {
  state.offices = null;
  state.rooms = null;
}

export function invalidateUsers() {
  state.users = null;
}

export function officeName(offices, officeId) {
  const o = offices.find((o) => o.id === officeId);
  return o ? o.location : `Office #${officeId}`;
}

export function roomLabel(room, offices) {
  return `Room #${room.room_id} — ${officeName(offices, room.office_id)} (${room.timezone})`;
}

export function userLabel(users, userId) {
  const u = users.find((u) => u.id === userId);
  return u ? `${u.name} <${u.email}>` : `User #${userId}`;
}
