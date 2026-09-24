// Dummy payment modal. Simulates a payment provider round-trip so the FE can
// exercise the real product flow: CreateBooking puts a slot InProgress and
// holds it, and only a successful payment triggers the confirm action that
// moves it to Booked. A declined/abandoned payment leaves the row InProgress
// exactly as it would be if the user walked away mid-checkout — the backend
// reaper is what eventually clears it, not this modal.

const overlay = document.getElementById("payment-modal");
const labelEl = document.getElementById("payment-label");
const amountEl = document.getElementById("payment-amount");
const errorEl = document.getElementById("payment-error");
const cardInput = document.getElementById("pm-card");
const expiryInput = document.getElementById("pm-expiry");
const cvvInput = document.getElementById("pm-cvv");
const payBtn = document.getElementById("pm-pay");
const cancelBtn = document.getElementById("pm-cancel");

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

cardInput.addEventListener("input", () => {
  const digits = cardInput.value.replace(/\D/g, "").slice(0, 16);
  cardInput.value = digits.replace(/(.{4})/g, "$1 ").trim();
});

function reset() {
  errorEl.innerHTML = "";
  cardInput.value = "";
  expiryInput.value = "";
  cvvInput.value = "";
  payBtn.disabled = false;
  payBtn.textContent = "Pay";
}

function close() {
  overlay.classList.add("hidden");
}

// amountCents: cosmetic only, purely for display. label: what's being paid
// for. onSuccess/onFailure/onCancel are called after the modal closes (for
// cancel) or while it's still open (for success/failure, so callers can
// decide whether to close it themselves after their own follow-up call).
export function openPaymentModal({ amountCents, label, onSuccess, onFailure, onCancel }) {
  reset();
  labelEl.textContent = label;
  amountEl.textContent = `$${(amountCents / 100).toFixed(2)}`;
  overlay.classList.remove("hidden");

  cancelBtn.onclick = () => {
    close();
    onCancel?.();
  };

  payBtn.onclick = async () => {
    errorEl.innerHTML = "";
    const cardDigits = cardInput.value.replace(/\D/g, "");
    if (cardDigits.length < 12) {
      errorEl.innerHTML = `<div class="error-banner">Enter a card number.</div>`;
      return;
    }

    payBtn.disabled = true;
    payBtn.textContent = "Processing…";
    await sleep(900); // simulate a network round-trip to a payment provider

    if (cardDigits.endsWith("0000")) {
      payBtn.disabled = false;
      payBtn.textContent = "Pay";
      errorEl.innerHTML = `<div class="error-banner">Payment declined. Your hold is still active — try a different card, or close this and it'll release automatically when the hold expires.</div>`;
      onFailure?.();
      return;
    }

    close();
    onSuccess?.();
  };
}
