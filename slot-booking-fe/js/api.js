const API_BASE = "/api";

export class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

async function request(path, { method = "GET", body } = {}) {
  const res = await fetch(API_BASE + path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (res.status === 204) return null;

  let data = null;
  const text = await res.text();
  if (text) {
    try { data = JSON.parse(text); } catch { data = null; }
  }

  if (!res.ok) {
    const message = (data && data.error) || `request failed with status ${res.status}`;
    throw new ApiError(res.status, message);
  }
  return data;
}

export const api = {
  health: () => request("/healthz"),

  listOffices: () => request("/offices"),
  createOffice: (location) => request("/offices", { method: "POST", body: { location } }),

  listRooms: () => request("/rooms"),
  createRoom: (officeId, timezone) =>
    request("/rooms", { method: "POST", body: { office_id: officeId, timezone } }),

  listUsers: () => request("/users"),
  createUser: (name, email) => request("/users", { method: "POST", body: { name, email } }),

  viewSlots: (roomId, date) => request(`/rooms/${roomId}/slots?date=${encodeURIComponent(date)}`),
  getAvailableSlots: (roomId, date, from, to) => {
    const params = new URLSearchParams({ date });
    if (from) params.set("from", from);
    if (to) params.set("to", to);
    return request(`/rooms/${roomId}/slots/available?${params}`);
  },

  createBooking: (body) => request("/bookings", { method: "POST", body }),
  updateBooking: (slotId, body) => request(`/bookings/${slotId}`, { method: "PATCH", body }),
  deleteBooking: (slotId) => request(`/bookings/${slotId}`, { method: "DELETE" }),
};
