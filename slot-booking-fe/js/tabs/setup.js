import { api, ApiError } from "../api.js";
import { getOffices, getRooms, invalidateOfficesAndRooms, officeName } from "../state.js";

function errorBanner(message) {
  return `<div class="error-banner">${message}</div>`;
}

export async function renderSetup(container) {
  container.innerHTML = `<div class="muted">Loading…</div>`;

  const [offices, rooms] = await Promise.all([getOffices(), getRooms()]);

  container.innerHTML = `
    <div class="panel">
      <h2>Offices</h2>
      <div id="office-error"></div>
      <form id="office-form" class="row" style="margin-bottom:14px">
        <div class="field">
          <label>Location</label>
          <input type="text" id="office-location" required placeholder="Blr HQ" />
        </div>
        <button type="submit" class="primary">Add office</button>
      </form>
      <table class="simple">
        <thead><tr><th>ID</th><th>Location</th></tr></thead>
        <tbody>
          ${offices.map((o) => `<tr><td>${o.id}</td><td>${o.location}</td></tr>`).join("") ||
            `<tr><td colspan="2" class="muted">No offices yet.</td></tr>`}
        </tbody>
      </table>
    </div>

    <div class="panel">
      <h2>Rooms</h2>
      <div id="room-error"></div>
      <form id="room-form" class="row" style="margin-bottom:14px">
        <div class="field">
          <label>Office</label>
          <select id="room-office" required>
            ${offices.map((o) => `<option value="${o.id}">${o.location}</option>`).join("")}
          </select>
        </div>
        <div class="field">
          <label>Timezone</label>
          <input type="text" id="room-timezone" value="UTC" placeholder="Asia/Kolkata" />
        </div>
        <button type="submit" class="primary" ${offices.length ? "" : "disabled"}>Add room</button>
      </form>
      <table class="simple">
        <thead><tr><th>Room ID</th><th>Office</th><th>Timezone</th></tr></thead>
        <tbody>
          ${rooms.map((r) => `<tr><td>${r.room_id}</td><td>${officeName(offices, r.office_id)}</td><td>${r.timezone}</td></tr>`).join("") ||
            `<tr><td colspan="3" class="muted">No rooms yet.</td></tr>`}
        </tbody>
      </table>
    </div>
  `;

  container.querySelector("#office-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const errBox = container.querySelector("#office-error");
    errBox.innerHTML = "";
    const location = container.querySelector("#office-location").value.trim();
    try {
      await api.createOffice(location);
      invalidateOfficesAndRooms();
      await renderSetup(container);
    } catch (err) {
      errBox.innerHTML = errorBanner(err instanceof ApiError ? err.message : "Failed to create office");
    }
  });

  container.querySelector("#room-form")?.addEventListener("submit", async (e) => {
    e.preventDefault();
    const errBox = container.querySelector("#room-error");
    errBox.innerHTML = "";
    const officeId = Number(container.querySelector("#room-office").value);
    const timezone = container.querySelector("#room-timezone").value.trim() || "UTC";
    try {
      await api.createRoom(officeId, timezone);
      invalidateOfficesAndRooms();
      await renderSetup(container);
    } catch (err) {
      errBox.innerHTML = errorBanner(err instanceof ApiError ? err.message : "Failed to create room");
    }
  });
}
