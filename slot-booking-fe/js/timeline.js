import { userLabel } from "./state.js";

function fmtTime(iso) {
  const d = new Date(iso);
  return d.toLocaleString(undefined, {
    month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
  });
}

function durationLabel(seconds) {
  const mins = Math.round(seconds / 60);
  if (mins < 60) return `${mins}m`;
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  return m ? `${h}h ${m}m` : `${h}h`;
}

// Renders a ViewSlots timeline into `container`. Free segments are clickable
// and invoke onFreeClick(entry) so the booking form can be pre-filled.
export function renderTimeline(container, timeline, users, onFreeClick) {
  container.innerHTML = "";
  const wrap = document.createElement("div");
  wrap.className = "timeline";

  if (!timeline.length) {
    wrap.innerHTML = `<div class="muted">Nothing in this window.</div>`;
    container.appendChild(wrap);
    return;
  }

  for (const entry of timeline) {
    const seg = document.createElement("div");
    if (entry.type === "free") {
      seg.className = "segment free";
      seg.innerHTML = `
        <div class="times">${fmtTime(entry.start_time)} – ${fmtTime(entry.end_time)}</div>
        <div class="meta">free — click to book</div>
      `;
      seg.addEventListener("click", () => onFreeClick(entry));
    } else {
      seg.className = "segment booking";
      const host = entry.host_user_id != null ? userLabel(users, entry.host_user_id) : "—";
      const participants = (entry.participant_ids || [])
        .map((id) => userLabel(users, id))
        .join(", ") || "none";
      seg.innerHTML = `
        <div>
          <div class="times">${fmtTime(entry.start_time)} (${durationLabel(entry.duration_seconds)})</div>
          <div class="meta">Host: ${host} · Participants: ${participants}</div>
        </div>
        <span class="badge ${entry.booking_state}">${entry.booking_state}</span>
      `;
    }
    wrap.appendChild(seg);
  }
  container.appendChild(wrap);
}
