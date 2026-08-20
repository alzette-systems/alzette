const screens = [...document.querySelectorAll('[data-screen]')];
const stateButtons = [...document.querySelectorAll('[data-show-state]')];
const authOnly = [...document.querySelectorAll('[data-auth-only]')];
const applicationRows = [...document.querySelectorAll('.screen[data-screen="launcher"] [data-app]')];
const launchButton = document.querySelector('[data-launch]');
const launchTitle = document.querySelector('[data-launch-title]');
const launchDetail = document.querySelector('[data-launch-detail]');
const catalogueToggle = document.querySelector('[data-catalogue-toggle]');
const modelDrawer = document.querySelector('[data-model-drawer]');
const modelDialog = document.querySelector('[data-model-dialog]');
const sessionDialog = document.querySelector('[data-session-dialog]');
const menuButton = document.querySelector('[data-menu-button]');
const accountMenu = document.querySelector('[data-account-menu]');
const signOutButton = document.querySelector('[data-sign-out]');
const cleanupBanner = document.querySelector('[data-cleanup-banner]');
const liveRegion = document.querySelector('[data-live-region]');
const toast = document.querySelector('[data-toast]');

const appCopy = {
  pi: { name: 'Pi', models: 4, mode: 'catalogue' },
  jan: { name: 'Jan Desktop', models: 4, mode: 'catalogue' },
  claude: { name: 'Claude Code', models: 2, mode: 'single' },
};

let selectedApp = 'pi';
let currentState = 'launcher';
let cleanupPending = false;
let toastTimer;

const stateMessages = {
  'signed-out': 'Signed out. Sign in with Alzette to continue.',
  launcher: 'Application launcher ready. Four models are available.',
  preparing: 'Preparing Pi and its secure local connection.',
  running: 'Pi is running through Alzette Connect.',
  recovery: 'Pi is disconnected. One local cleanup action needs attention.',
};

function showToast(message) {
  if (!toast) return;
  window.clearTimeout(toastTimer);
  toast.textContent = message;
  toast.hidden = false;
  toastTimer = window.setTimeout(() => { toast.hidden = true; }, 3200);
}

function showState(state, { focus = true } = {}) {
  currentState = state;
  screens.forEach((screen) => { screen.hidden = screen.dataset.screen !== state; });
  stateButtons.forEach((button) => {
    const active = button.dataset.showState === state;
    button.classList.toggle('is-active', active);
    button.setAttribute('aria-current', active ? 'true' : 'false');
  });
  authOnly.forEach((node) => { node.hidden = state === 'signed-out'; });
  if (cleanupBanner) cleanupBanner.hidden = !cleanupPending;
  if (signOutButton) {
    signOutButton.textContent = state === 'running'
      ? 'Disconnect and sign out'
      : state === 'preparing' ? 'Cancel launch and sign out' : 'Sign out';
  }
  if (state !== 'launcher') closeCatalogue();
  if (liveRegion) liveRegion.textContent = stateMessages[state] || '';
  const url = new URL(window.location.href);
  url.searchParams.set('state', state);
  history.replaceState({}, '', url);
  if (focus) {
    window.requestAnimationFrame(() => {
      const heading = document.querySelector(`[data-screen="${state}"] h1`);
      if (heading) {
        heading.tabIndex = -1;
        heading.focus();
      }
    });
  }
}

function closeCatalogue() {
  if (!modelDrawer || !catalogueToggle) return;
  modelDrawer.hidden = true;
  catalogueToggle.setAttribute('aria-expanded', 'false');
}

function selectApplication(row) {
  if (!row || row.disabled || row.getAttribute('aria-disabled') === 'true') return;
  const app = appCopy[row.dataset.app];
  if (!app) return;
  selectedApp = row.dataset.app;
  applicationRows.forEach((candidate) => {
    const selected = candidate === row;
    candidate.classList.toggle('is-selected', selected);
    candidate.setAttribute('aria-selected', selected ? 'true' : 'false');
  });
  if (app.mode === 'single') {
    launchTitle.textContent = `Choose a model, then launch ${app.name}`;
    launchDetail.textContent = `${app.name} requires one model at launch. ${app.models} assigned models are compatible.`;
    launchButton.textContent = 'Choose model';
  } else {
    launchTitle.textContent = `Launch ${app.name} through Alzette`;
    launchDetail.textContent = `${app.name} will receive all ${app.models} compatible models. Choose a model inside ${app.name}.`;
    launchButton.textContent = `Launch ${app.name}`;
  }
}

function launchSelected() {
  const app = appCopy[selectedApp];
  if (!app) return;
  if (app.mode === 'single') {
    modelDialog.showModal();
    return;
  }
  showState('preparing');
}

stateButtons.forEach((button) => button.addEventListener('click', () => showState(button.dataset.showState)));

applicationRows.forEach((row) => {
  row.addEventListener('click', () => selectApplication(row));
  row.addEventListener('dblclick', () => {
    if (row.disabled || row.getAttribute('aria-disabled') === 'true') return;
    selectApplication(row);
    launchSelected();
  });
  row.addEventListener('keydown', (event) => {
    if (event.key === 'Enter' && !row.disabled && row.getAttribute('aria-disabled') !== 'true') {
      event.preventDefault();
      selectApplication(row);
      launchSelected();
    }
  });
});

launchButton?.addEventListener('click', launchSelected);
document.querySelector('[data-sign-in]')?.addEventListener('click', () => showState('launcher'));
signOutButton?.addEventListener('click', () => {
  accountMenu.hidden = true;
  menuButton.setAttribute('aria-expanded', 'false');
  if (currentState === 'running' || currentState === 'preparing') {
    sessionDialog.showModal();
    return;
  }
  showState('signed-out');
});
document.querySelector('[data-confirm-sign-out]')?.addEventListener('click', () => {
  cleanupPending = false;
  window.setTimeout(() => showState('signed-out'), 0);
});
document.querySelector('[data-cancel-launch]')?.addEventListener('click', () => showState('launcher'));
document.querySelector('[data-disconnect]')?.addEventListener('click', () => { cleanupPending = true; showState('recovery'); });
document.querySelector('[data-back-launcher]')?.addEventListener('click', () => { cleanupPending = true; showState('launcher'); });
document.querySelector('[data-review-cleanup]')?.addEventListener('click', () => showState('recovery'));
document.querySelector('[data-resolve-cleanup]')?.addEventListener('click', () => {
  cleanupPending = false;
  showState('launcher');
  showToast('Pi profile restored. Cleanup is complete.');
});
document.querySelector('[data-hide-tray]')?.addEventListener('click', () => showToast('Pi is still connected. Connect would now remain active in the system tray.'));
document.querySelector('[data-diagnostics]')?.addEventListener('click', () => {
  accountMenu.hidden = true;
  menuButton.setAttribute('aria-expanded', 'false');
  showToast('Diagnostics preview: connection healthy; no secrets included.');
});
document.querySelector('[data-single-preview]')?.addEventListener('click', () => {
  showState('launcher');
  modelDialog.showModal();
});
document.querySelector('[data-launch-single]')?.addEventListener('click', () => {
  window.setTimeout(() => showToast('Future adapter preview only — no application was launched.'), 0);
});

catalogueToggle?.addEventListener('click', () => {
  const expanded = catalogueToggle.getAttribute('aria-expanded') === 'true';
  modelDrawer.hidden = expanded;
  catalogueToggle.setAttribute('aria-expanded', expanded ? 'false' : 'true');
});

menuButton?.addEventListener('click', () => {
  accountMenu.hidden = !accountMenu.hidden;
  menuButton.setAttribute('aria-expanded', accountMenu.hidden ? 'false' : 'true');
});
document.addEventListener('click', (event) => {
  if (!accountMenu.hidden && !accountMenu.contains(event.target) && !menuButton.contains(event.target)) {
    accountMenu.hidden = true;
    menuButton.setAttribute('aria-expanded', 'false');
  }
});

const params = new URLSearchParams(window.location.search);
if (params.get('capture') === '1') document.body.dataset.capture = 'true';
const initialState = params.get('state') || 'launcher';
showState(screens.some((screen) => screen.dataset.screen === initialState) ? initialState : 'launcher', { focus: false });
selectApplication(applicationRows.find((row) => row.dataset.app === selectedApp));
