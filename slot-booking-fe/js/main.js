import { getCurrentUser, loginOrRegister, logout } from "./auth.js";
import { ApiError } from "./api.js";
import { renderSetup } from "./tabs/setup.js";
import { renderBrowse } from "./tabs/browse.js";
import { renderUpcoming } from "./tabs/upcoming.js";
import { renderManage } from "./tabs/manage.js";

const loginView = document.getElementById("login-view");
const appView = document.getElementById("app-view");
const loginForm = document.getElementById("login-form");
const loginError = document.getElementById("login-error");
const whoName = document.getElementById("who-name");

const TABS = {
  setup: renderSetup,
  browse: renderBrowse,
  upcoming: renderUpcoming,
  manage: renderManage,
};

function showApp(user) {
  loginView.classList.add("hidden");
  appView.classList.remove("hidden");
  whoName.textContent = `${user.name} <${user.email}>`;
  activateTab("setup");
}

function showLogin() {
  appView.classList.add("hidden");
  loginView.classList.remove("hidden");
}

async function activateTab(name) {
  document.querySelectorAll("nav.tabs button").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.tab === name);
  });
  document.querySelectorAll(".tab-panel").forEach((panel) => {
    panel.classList.toggle("hidden", panel.id !== `tab-${name}`);
  });
  const panel = document.getElementById(`tab-${name}`);
  try {
    await TABS[name](panel);
  } catch (err) {
    panel.innerHTML = `<div class="error-banner">${err instanceof ApiError ? err.message : "Something went wrong loading this tab."}</div>`;
  }
}

document.querySelectorAll("nav.tabs button").forEach((btn) => {
  btn.addEventListener("click", () => activateTab(btn.dataset.tab));
});

document.getElementById("logout-btn").addEventListener("click", () => {
  logout();
  showLogin();
});

loginForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  loginError.innerHTML = "";
  const name = document.getElementById("login-name").value;
  const email = document.getElementById("login-email").value;
  try {
    const user = await loginOrRegister(name, email);
    showApp(user);
  } catch (err) {
    loginError.innerHTML = `<div class="error-banner">${err instanceof ApiError ? err.message : "Login failed — is the backend running?"}</div>`;
  }
});

const existing = getCurrentUser();
if (existing) {
  showApp(existing);
} else {
  showLogin();
}
