import { api, ApiError } from "../api.js";
import { getOffices, getRooms, getUsers, roomLabel } from "../state.js";
import { renderTimeline } from "../timeline.js";
import { getCurrentUser } from "../auth.js";
import { openPaymentModal } from "../payment.js";

// Cosmetic only — dummy rate used to show a plausible-looking charge in the
// payment modal.
const CENTS_PER_MINUTE = 20;

const DURATIONS = [
  [15 * 60, "15 min"],
  [30 * 60, "30 min"],
  [45 * 60, "45 min"],
  [60 * 60, "1 hour"],
  [90 * 60, "1.5 hours"],
];

function todayLocal() {
  const d = new Date();
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 10);
}

function errorBanner(message) {
  return `<div class="error-banner">${message}</div>`;
}

function infoBanner(message) {
  return `<div class="info-banner">${message}</div>`;
}

export async function renderBrowse(container) {
  container.innerHTML = `<div class="muted">Loading…</div>`;
  const [offices, rooms] = await Promise.all([getOffices(), getRooms()]);

  if (!rooms.length) {
    container.innerHTML = `<div class="panel">${infoBanner("No rooms yet — add one in the Setup tab first.")}</div>`;
    return;
  }

  container.innerHTML = `
    <div class="panel">
      <h2>Pick a room and date</h2>
      <div class="row">
        <div class="field">
          <label>Room</label>
          <select id="browse-room">
            ${rooms.map((r) => `<option value="${r.room_id}">${roomLabel(r, offices)}</option>`).join("")}
          </select>
        </div>
        <div class="field">
          <label>Date</label>
          <input type="date" id="browse-date" value="${todayLocal()}" />
        </div>
        <button id="browse-load" class="primary">View timeline</button>
      </div>
    </div>

    <div class="panel">
      <h2>Timeline</h2>
      <div id="browse-timeline"><div class="muted">Pick a room and date, then click "View timeline".</div></div>
    </div>

    <div class="panel">
      <h2>Book a slot</h2>
      <div id="book-msg"></div>
      <form id="book-form">
        <div class="row">
          <div class="field">
            <label>Start time (on the selected date, your local time)</label>
            <input type="time" id="book-time" required />
          </div>
          <div class="field">
            <label>Duration</label>
            <select id="book-duration">
              ${DURATIONS.map(([secs, label]) => `<option value="${secs}">${label}</option>`).join("")}
            </select>
          </div>
        </div>
        <div class="field" style="margin-top:10px">
          <label>Participant emails (comma-separated, optional — each must already have an account)</label>
          <input type="text" id="book-participants" placeholder="ada@example.com, alan@example.com" />
        </div>
        <div style="margin-top:12px">
          <button type="submit" class="primary">Book</button>
        </div>
      </form>
    </div>
  `;

  const roomSelect = container.querySelector("#browse-room");
  const dateInput = container.querySelector("#browse-date");
  const timelineBox = container.querySelector("#browse-timeline");
  const bookMsg = container.querySelector("#book-msg");
  const timeInput = container.querySelector("#book-time");

  async function loadTimeline() {
    const roomId = roomSelect.value;
    const date = dateInput.value;
    if (!date) return;
    timelineBox.innerHTML = `<div class="muted">Loading…</div>`;
    try {
      const users = await getUsers();
      const data = await api.viewSlots(roomId, date);
      renderTimeline(timelineBox, data.timeline, users, (entry) => {
        const d = new Date(entry.start_time);
        timeInput.value = d.toTimeString().slice(0, 5);
      });
    } catch (err) {
      timelineBox.innerHTML = errorBanner(err instanceof ApiError ? err.message : "Failed to load timeline");
    }
  }

  container.querySelector("#browse-load").addEventListener("click", loadTimeline);
  roomSelect.addEventListener("change", loadTimeline);
  dateInput.addEventListener("change", loadTimeline);
  loadTimeline();

  container.querySelector("#book-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    bookMsg.innerHTML = "";

    const me = getCurrentUser();
    const roomId = Number(roomSelect.value);
    const date = dateInput.value;
    const time = timeInput.value;
    const durationSeconds = Number(container.querySelector("#book-duration").value);
    const emailsRaw = container.querySelector("#book-participants").value.trim();

    if (!date || !time) {
      bookMsg.innerHTML = errorBanner("Pick a date and start time.");
      return;
    }

    let participantIds = [];
    if (emailsRaw) {
      const users = await getUsers(true);
      const emails = emailsRaw.split(",").map((s) => s.trim().toLowerCase()).filter(Boolean);
      const unresolved = [];
      for (const email of emails) {
        const u = users.find((u) => u.email.toLowerCase() === email);
        if (u) participantIds.push(u.id);
        else unresolved.push(email);
      }
      if (unresolved.length) {
        bookMsg.innerHTML = errorBanner(
          `No account found for: ${unresolved.join(", ")}. They need to log in once to create an account.`
        );
        return;
      }
    }

    // Interpreted in the browser's local timezone, not the room's IANA
    // timezone field — a real implementation would need Intl-based tz math.
    const startTime = new Date(`${date}T${time}`).toISOString();

    let booking;
    try {
      booking = await api.createBooking({
        room_id: roomId,
        start_time: startTime,
        duration_seconds: durationSeconds,
        host_user_id: me.id,
        participant_user_ids: participantIds,
      });
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        bookMsg.innerHTML = errorBanner("That time overlaps an existing booking for this room.");
      } else {
        bookMsg.innerHTML = errorBanner(err instanceof ApiError ? err.message : "Failed to create booking");
      }
      return;
    }

    // The slot is now held InProgress. It only becomes Booked once the
    // (dummy) payment succeeds and we call confirm — if the payment fails or
    // the user abandons checkout, it stays InProgress until the backend
    // reaper releases the hold, exactly like a real abandoned checkout.
    bookMsg.innerHTML = infoBanner("Slot held — complete payment to confirm your booking.");
    loadTimeline();

    openPaymentModal({
      amountCents: Math.round((durationSeconds / 60) * CENTS_PER_MINUTE),
      label: `Room #${roomId} booking — ${date} ${time}`,
      onSuccess: async () => {
        try {
          await api.updateBooking(booking.slot_id, {
            action: "confirm",
            host_user_id: me.id,
            participant_user_ids: participantIds,
          });
          bookMsg.innerHTML = infoBanner("Payment successful — booking confirmed!");
          container.querySelector("#book-participants").value = "";
        } catch (err) {
          if (err instanceof ApiError && (err.status === 410 || err.status === 404)) {
            bookMsg.innerHTML = errorBanner("Payment went through, but the hold had already expired — please try booking again.");
          } else {
            bookMsg.innerHTML = errorBanner(err instanceof ApiError ? err.message : "Payment succeeded but confirming the booking failed.");
          }
        }
        loadTimeline();
      },
      onFailure: () => {
        bookMsg.innerHTML = errorBanner("Payment declined. Your slot is still held — retry from this modal, or from Manage Bookings before the hold expires.");
      },
      onCancel: () => {
        bookMsg.innerHTML = infoBanner("Checkout closed. Your slot is still held InProgress — finish payment from Manage Bookings before it expires.");
        loadTimeline();
      },
    });
  });
}
