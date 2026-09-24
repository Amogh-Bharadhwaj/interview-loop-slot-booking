import { api, ApiError } from "../api.js";
import { getOffices, getRooms, getUsers, roomLabel, userLabel } from "../state.js";
import { scanBookings } from "../bookingScan.js";
import { getCurrentUser } from "../auth.js";
import { openPaymentModal } from "../payment.js";

const CENTS_PER_MINUTE = 20;

function fmt(iso) {
  return new Date(iso).toLocaleString(undefined, {
    weekday: "short", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
  });
}

function errorBanner(message) {
  return `<div class="error-banner">${message}</div>`;
}

export async function renderManage(container) {
  container.innerHTML = `<div class="muted">Scanning your rooms for bookings you created…</div>`;

  const me = getCurrentUser();
  const [offices, rooms, users] = await Promise.all([getOffices(), getRooms(), getUsers()]);

  const entries = await scanBookings(rooms, {
    days: 7,
    matches: (e) => e.host_user_id === me.id && e.booking_state !== "Completed",
  });

  const roomsById = new Map(rooms.map((r) => [r.room_id, r]));

  container.innerHTML = `
    <div class="panel">
      <h2>Bookings you created</h2>
      <div class="muted" style="margin-bottom:10px">Confirm before the hold expires, edit participants, or cancel.</div>
      <div id="manage-msg"></div>
      <div class="list" id="manage-list">
        ${entries.length ? "" : `<div class="muted">You haven't created any bookings in the next 7 days.</div>`}
      </div>
    </div>
  `;

  const msg = container.querySelector("#manage-msg");
  const list = container.querySelector("#manage-list");

  for (const e of entries) {
    const room = roomsById.get(e.room_id);
    const participantEmails = (e.participant_ids || [])
      .map((id) => users.find((u) => u.id === id)?.email)
      .filter(Boolean)
      .join(", ");

    const item = document.createElement("div");
    item.className = "list-item";
    item.innerHTML = `
      <div style="flex:1;min-width:220px">
        <div><strong>${fmt(e.start_time)}</strong> — ${room ? roomLabel(room, offices) : `Room #${e.room_id}`}
          <span class="badge ${e.booking_state}">${e.booking_state}</span>
        </div>
        <div class="muted">Participants: ${(e.participant_ids || []).map((id) => userLabel(users, id)).join(", ") || "none"}</div>
        <div class="field" style="margin-top:6px">
          <input type="text" class="edit-participants" value="${participantEmails}" placeholder="comma-separated emails" />
        </div>
      </div>
      <div class="actions">
        ${e.booking_state === "InProgress" ? `<button class="secondary confirm-btn">Pay &amp; confirm</button>` : ""}
        <button class="secondary save-btn">Save participants</button>
        <button class="danger delete-btn">Delete</button>
      </div>
    `;
    list.appendChild(item);

    const emailInput = item.querySelector(".edit-participants");

    async function resolveParticipants() {
      const raw = emailInput.value.trim();
      if (!raw) return [];
      const freshUsers = await getUsers(true);
      const emails = raw.split(",").map((s) => s.trim().toLowerCase()).filter(Boolean);
      const ids = [];
      const unresolved = [];
      for (const email of emails) {
        const u = freshUsers.find((u) => u.email.toLowerCase() === email);
        if (u) ids.push(u.id);
        else unresolved.push(email);
      }
      if (unresolved.length) {
        throw new Error(`No account found for: ${unresolved.join(", ")}`);
      }
      return ids;
    }

    item.querySelector(".confirm-btn")?.addEventListener("click", async () => {
      msg.innerHTML = "";
      let participantIds;
      try {
        participantIds = await resolveParticipants();
      } catch (err) {
        msg.innerHTML = errorBanner(err.message || "Failed to resolve participants");
        return;
      }

      openPaymentModal({
        amountCents: Math.round((e.duration_seconds / 60) * CENTS_PER_MINUTE),
        label: `Room #${e.room_id} booking — ${fmt(e.start_time)}`,
        onSuccess: async () => {
          try {
            await api.updateBooking(e.slot_id, {
              action: "confirm",
              host_user_id: me.id,
              participant_user_ids: participantIds,
            });
            await renderManage(container);
          } catch (err) {
            if (err instanceof ApiError && (err.status === 410 || err.status === 404)) {
              msg.innerHTML = errorBanner("That hold already expired and was released.");
              await renderManage(container);
            } else {
              msg.innerHTML = errorBanner(err.message || "Failed to confirm booking");
            }
          }
        },
        onFailure: () => {
          msg.innerHTML = errorBanner("Payment declined. The hold is still active — retry, or it'll release automatically when it expires.");
        },
      });
    });

    item.querySelector(".save-btn").addEventListener("click", async () => {
      msg.innerHTML = "";
      try {
        const participantIds = await resolveParticipants();
        await api.updateBooking(e.slot_id, {
          action: "update_participants",
          host_user_id: me.id,
          participant_user_ids: participantIds,
        });
        await renderManage(container);
      } catch (err) {
        msg.innerHTML = errorBanner(err.message || "Failed to update participants");
      }
    });

    item.querySelector(".delete-btn").addEventListener("click", async () => {
      if (!confirm("Delete this booking?")) return;
      msg.innerHTML = "";
      try {
        await api.deleteBooking(e.slot_id);
        await renderManage(container);
      } catch (err) {
        msg.innerHTML = errorBanner(err instanceof ApiError ? err.message : "Failed to delete booking");
      }
    });
  }
}
