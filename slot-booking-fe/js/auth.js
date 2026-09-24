import { api } from "./api.js";

const STORAGE_KEY = "sb_current_user";

export function getCurrentUser() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

function setCurrentUser(user) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(user));
}

export function logout() {
  localStorage.removeItem(STORAGE_KEY);
}

// The backend has no auth layer: identity is self-declared. "Logging in" is
// a lookup-or-create against the real Users API, keyed by email.
export async function loginOrRegister(name, email) {
  const users = await api.listUsers();
  const normalized = email.trim().toLowerCase();
  let user = users.find((u) => u.email.toLowerCase() === normalized);
  if (!user) {
    user = await api.createUser(name.trim(), email.trim());
  }
  setCurrentUser(user);
  return user;
}
