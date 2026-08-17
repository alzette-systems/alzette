(function () {
  'use strict';

  var doc = document;
  var modeMeta = doc.querySelector('meta[name="alzette-api-mode"]');
  var apiEnabled = !!modeMeta && modeMeta.getAttribute('content') === 'live';
  var hasPortal = !!doc.querySelector('#portal-main');
  var paths = {
    overview: '/app/overview',
    models: '/app/models',
    endpoints: '/app/endpoints',
    usage: '/app/usage',
    access: '/app/access',
    billing: '/app/billing',
    docs: '/app/docs'
  };
  var state = {
    live: apiEnabled,
    me: null,
    dashboard: null,
    access: null,
    catalogue: null,
    endpoints: null,
    modelDetail: null,
    endpointDetail: null,
    requestProgress: null,
    billing: null,
    resource: null,
    configurator: {
      step: 1,
      modelSlug: '',
      mode: '',
      profileId: '',
      draftId: '',
      draftLoaded: false,
      operationId: '',
      values: {}
    },
    csrfToken: '',
    routeSelection: '',
    filters: {},
    keyAction: 'overlap',
    keyTarget: null,
    oneTimeSecret: '',
    operationKeys: {},
    pendingCommercialAction: null,
    capacityTarget: null,
    loadGeneration: 0,
    resourceGeneration: 0,
    loading: false,
    pageError: null
  };

  var FALLBACK = {
    account: { name: 'Client organisation', initials: 'CO' },
    scope: { organization: 'Client organisation', project: 'Project', environment: 'Environment', projectEnvironment: 'Project / environment' },
    source: {
      kind: 'Target registry + inference ledger',
      label: 'Illustrative preview — not live account data.',
      detail: 'Connect an authenticated project/environment snapshot to replace this safe shell; no active probe is running.',
      asOf: null,
      freshness: 'Unknown',
      finality: 'Unknown'
    },
    route: {
      state: 'unknown',
      statusLabel: 'Callability is not confirmed',
      statusDetail: 'No target registry or latest inference observation is connected; no active probe is running.',
      attentionTitle: 'No active route probe',
      attentionDetail: 'This surface reports the target registry plus the latest inference observation; it does not run a live health probe.',
      lastSuccessAt: null,
      lastObservationAt: null,
      modelAlias: null,
      executionClass: 'Execution boundary unavailable · static fallback',
      capacityMode: 'Service mode unavailable · static fallback',
      boundaryHeadline: 'Execution and service mode unavailable · static fallback',
      boundaryDetail: 'Static fallback semantics only. A connected view hydrates this from the selected route and service plan.',
      capacityHeadline: 'Execution and capacity details are unavailable.'
    },
    usage: {
      logicalRequests: null,
      successfulRequests: null,
      failedRequests: null,
      blockedRequests: null,
      successRate: null,
      errorRate: null,
      p95LatencyMs: null,
      throughput: null,
      peakConcurrency: null,
      tokens: { input: null, output: null, cached: null, reasoning: null, total: null },
      tokenFinality: null
    },
    allocation: {
      shared: 'Unknown — no contractual value supplied',
      dedicated: 'Unknown — no contractual value supplied',
      source: 'Contract source unavailable',
      finality: 'Unknown',
      contextDetail: 'Allowance and allocation values appear only when an authoritative contract source supplies them.'
    },
    period: { label: 'Current period', timezone: 'Timezone not supplied' },
    trend: { unit: 'Unit unavailable', points: [] },
    breakdowns: { projects: [], models: [], serviceAccounts: [] },
    requests: [],
    routes: [],
    ambiguous: false,
    gatewayBaseUrl: null,
    exportMeta: { available: false, formats: [] },
    attribution: { serviceAccount: 'Unavailable in this snapshot' }
  };

  FALLBACK.catalogue = { unavailable: true, source: 'Catalogue unavailable', detail: 'This illustrative preview has no connected model registry.', models: [] };
  FALLBACK.endpoints = { unavailable: true, source: 'Endpoint inventory unavailable', detail: 'This illustrative preview has no connected endpoint inventory.', endpoints: [] };
  FALLBACK.billing = { unavailable: true, state: 'not_configured', source: 'Billing unavailable', detail: 'This illustrative preview has no connected billing record.', invoices: [] };

  function q(selector, root) {
    return (root || doc).querySelector(selector);
  }

  function all(selector, root) {
    return Array.prototype.slice.call((root || doc).querySelectorAll(selector));
  }

  function create(tag, className, text) {
    var node = doc.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  function removeChildren(node) {
    if (!node) return;
    while (node.firstChild) node.removeChild(node.firstChild);
  }

  function get(object, path) {
    if (!object || !path) return undefined;
    return path.split('.').reduce(function (value, key) {
      return value === undefined || value === null ? undefined : value[key];
    }, object);
  }

  function firstValue(object, candidates) {
    var list = Array.isArray(candidates) ? candidates : [candidates];
    for (var i = 0; i < list.length; i += 1) {
      var value = typeof list[i] === 'string' ? get(object, list[i]) : list[i];
      if (value !== undefined && value !== null && value !== '') return value;
    }
    return null;
  }

  function arrayValue(value) {
    if (Array.isArray(value)) return value;
    if (value && typeof value === 'object') return Object.keys(value).map(function (key) { return value[key]; });
    return [];
  }

  function unwrap(payload) {
    if (!payload || typeof payload !== 'object') return {};
    if (payload.data && typeof payload.data === 'object' && !Array.isArray(payload.data)) return payload.data;
    return payload;
  }

  function stringValue(value, fallback) {
    if (value === undefined || value === null || value === '') return fallback;
    return String(value);
  }

  function numberValue(value) {
    if (value === undefined || value === null || value === '' || Number.isNaN(Number(value))) return null;
    return Number(value);
  }

  function formatCount(value) {
    var number = numberValue(value);
    return number === null ? 'Unknown' : new Intl.NumberFormat().format(number);
  }

  function formatPercent(value) {
    var number = numberValue(value);
    if (number === null) return 'Unknown';
    if (Math.abs(number) <= 1) number *= 100;
    return number.toFixed(number % 1 === 0 ? 0 : 1) + '%';
  }

  function formatMilliseconds(value) {
    var number = numberValue(value);
    return number === null ? 'Unavailable' : new Intl.NumberFormat().format(number) + ' ms';
  }

  function formatThroughput(value) {
    var amount = value && typeof value === 'object' ? firstValue(value, ['value', 'rps', 'requests_per_second']) : value;
    var number = numberValue(amount);
    return number === null ? 'Unavailable' : new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(number) + ' rps';
  }

  function formatConcurrency(value) {
    var amount = value && typeof value === 'object' ? firstValue(value, ['value', 'count', 'peak']) : value;
    return formatCount(amount);
  }

  function formatTokenFinality(value) {
    var key = String(value || 'unknown').toLowerCase().replace(/[\s-]+/g, '_');
    var labels = { not_applicable: 'Not applicable', partial: 'Partial', unknown: 'Unknown', final: 'Final', complete: 'Final', completed: 'Final' };
    if (labels[key]) return labels[key];
    return key.split('_').filter(Boolean).map(function (part) { return part.charAt(0).toUpperCase() + part.slice(1); }).join(' ') || 'Unknown';
  }

  function formatPeriodBound(value) {
    if (!value) return '';
    var raw = String(value);
    var dateOnly = raw.match(/^\d{4}-\d{2}-\d{2}/);
    return dateOnly ? dateOnly[0] : formatDate(value);
  }

  function periodLabel(root) {
    var period = firstValue(root, ['period']) || {};
    var label = firstValue(root, ['period.label', 'period_label']);
    if (label !== null) return String(label);
    var fromRaw = firstValue(period, ['from', 'start', 'from_date']);
    var toRaw = firstValue(period, ['to', 'end', 'to_date']);
    var from = periodInputValue(fromRaw, false) || formatPeriodBound(fromRaw);
    var to = periodInputValue(toRaw, true) || formatPeriodBound(toRaw);
    if (from && to) return from + ' – ' + to;
    if (from) return 'From ' + from;
    if (to) return 'Through ' + to;
    return 'Current period';
  }

  function periodInputValue(value, exclusiveEnd) {
    if (!value) return '';
    var date = new Date(value);
    if (Number.isNaN(date.getTime())) return '';
    if (exclusiveEnd && date.getUTCHours() === 0 && date.getUTCMinutes() === 0 && date.getUTCSeconds() === 0 && date.getUTCMilliseconds() === 0) date.setUTCDate(date.getUTCDate() - 1);
    return date.toISOString().slice(0, 10);
  }

  function initializePeriodFilters(period) {
    var from = q('#usage-from');
    var to = q('#usage-to');
    if (from && !from.value && !state.filters.from) from.value = periodInputValue(period && period.from, false);
    if (to && !to.value && !state.filters.to) to.value = periodInputValue(period && period.to, true);
  }

  function formatDate(value) {
    if (!value) return 'Unknown';
    var date = new Date(value);
    if (Number.isNaN(date.getTime())) return String(value);
    return new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'UTC' }).format(date) + ' UTC';
  }

  function isoDateTime(value) {
    if (!value) return '';
    var date = new Date(value);
    return Number.isNaN(date.getTime()) ? '' : date.toISOString();
  }

  function formatList(value) {
    var list = arrayValue(value).filter(function (item) { return item !== undefined && item !== null && item !== ''; });
    return list.length ? list.join(', ') : 'Unknown';
  }

  function normalizeGateway(value) {
    if (!value || typeof value !== 'string') return null;
    try {
      var url = new URL(value.trim());
      if (url.protocol !== 'http:' && url.protocol !== 'https:') return null;
      return url.toString().replace(/\/$/, '');
    } catch (error) {
      return null;
    }
  }

  function bind(name, value) {
    var output = value === undefined || value === null || value === '' ? 'Unknown' : String(value);
    all('[data-bind="' + name + '"]').forEach(function (node) {
      node.textContent = output;
    });
  }

  function bindTime(name, value) {
    var output = formatDate(value);
    all('[data-bind="' + name + '"]').forEach(function (node) {
      node.textContent = output;
      if (node.tagName === 'TIME') node.setAttribute('datetime', isoDateTime(value));
    });
  }

  function setState(node, value) {
    if (node) node.setAttribute('data-state', value);
  }

  function setHidden(node, hidden) {
    if (node) node.hidden = !!hidden;
  }

  function showToast(message) {
    var toast = q('#portal-toast');
    if (!toast) return;
    toast.textContent = message;
    toast.hidden = false;
    window.clearTimeout(showToast.timer);
    showToast.timer = window.setTimeout(function () { toast.hidden = true; }, 4200);
  }

  function showTransportNotice() {
    if (window.location.protocol === 'https:') return;
    all('[data-transport-notice]').forEach(function (notice) {
      notice.hidden = false;
    });
  }

  function humanStatus(value) {
    var key = String(value || 'unknown').toLowerCase().replace(/[-\s]+/g, '_');
    var labels = {
      ready: 'Ready', active: 'Active', healthy: 'Ready', operational: 'Ready', running: 'Running', fresh: 'Fresh', degraded: 'Degraded', unavailable: 'Unavailable',
      stale: 'Stale', unknown: 'Unknown', pending: 'Pending', failed: 'Failed', paused: 'Paused',
      rate_limited: 'Rate limited', budget_blocked: 'Budget blocked', testing: 'Testing', provisioning: 'Provisioning',
      available_to_configure: 'Available to configure', payment_not_configured: 'Payment not configured', runtime_unavailable: 'Runtime unavailable', not_started: 'Not started'
    };
    return labels[key] || String(value || 'Unknown');
  }

  function humanExecution(value, evidenced) {
    if (value === undefined || value === null || value === '') return null;
    var key = String(value).toLowerCase().replace(/[\s-]+/g, '_');
    if (key === 'external_pilot' || key === 'external_pilot_via_openrouter' || key.indexOf('openrouter') >= 0 || key === 'shared' || key === 'shared_pilot') return 'Shared external execution';
    if (key === 'private_compatible' || key === 'private_compatible_target') return evidenced ? 'Private-compatible execution' : 'Private execution not yet evidenced';
    if (key === 'dedicated' || key === 'dedicated_allocation' || key === 'dedicated_private') return evidenced ? 'Dedicated private execution' : 'Dedicated execution not yet evidenced';
    if (key === 'unknown' || key === 'unavailable') return humanStatus(key);
    return 'Execution boundary not evidenced';
  }

  function humanCapacity(value, evidenced) {
    if (value === undefined || value === null || value === '') return null;
    var key = String(value).toLowerCase().replace(/[\s-]+/g, '_');
    if (key === 'shared' || key === 'shared_pilot') return 'Shared service';
    if (key === 'dedicated' || key === 'dedicated_allocation') return evidenced ? 'Dedicated allocation' : 'Dedicated allocation not yet evidenced';
    if (key === 'unavailable' || key === 'unknown') return humanStatus(key);
    return 'Capacity mode not evidenced';
  }

  function executionEvidencePresent(item, servicePlan) {
    return !!firstValue(item || {}, ['execution_evidence_ref', 'executionEvidenceRef', 'evidence.execution_ref', 'evidence.execution', 'allocation_evidence_ref', 'allocationEvidenceRef']) ||
      !!firstValue(servicePlan || {}, ['execution_evidence_ref', 'executionEvidenceRef', 'evidence.execution_ref', 'allocation_evidence_ref']);
  }

  function routeEvidence(route) {
    if (!route) return { state: 'Unavailable', label: 'Route evidence unavailable', detail: 'No selected route was supplied.', freshness: 'Unknown', source: 'Route evidence unavailable', noteTitle: 'Route evidence unavailable', noteDetail: 'Connect a selected route to see probe state and evidence freshness.' };
    var freshnessRaw = String(route.freshness || '').trim();
    var freshness = freshnessRaw.toLowerCase().replace(/[\s-]+/g, '_');
    var freshnessUnknown = !freshness || ['unknown', 'unavailable', 'not_available'].indexOf(freshness) >= 0;
    var freshUntilMs = route.freshUntil ? new Date(route.freshUntil).getTime() : NaN;
    var staleByTime = Number.isFinite(freshUntilMs) && freshUntilMs < Date.now();
    var stale = freshness.indexOf('stale') >= 0 || staleByTime;
    var optedIn = route.probeEnabled === true;
    var probeState = String(route.probeStatus || '').trim().toLowerCase().replace(/[\s-]+/g, '_');
    var probeUnavailable = probeState === 'unavailable';
    var probeUnknown = !probeState || ['unknown', 'not_available', 'not_configured', 'not_running', 'disabled', 'pending'].indexOf(probeState) >= 0;
    var probeReady = ['ready', 'healthy', 'operational', 'success', 'succeeded', 'ok'].indexOf(probeState) >= 0;
    var freshnessKnown = !freshnessUnknown && !stale;
    var registry = humanStatus(route.registryStatus || route.state);
    var registryUnavailable = String(route.registryStatus || route.state || '').toLowerCase().replace(/[\s-]+/g, '_') === 'unavailable';
    var latest = String(route.latestInferenceStatus || '').toLowerCase();
    var observation = latest && latest !== 'unknown' ? 'Latest inference observation: ' + humanStatus(latest) + '. ' : '';
    var source = optedIn ? 'Target registry + opted-in route probe' : 'Target registry + latest inference observation';
    var freshnessLabel = freshnessUnknown ? 'Unknown' : humanStatus(freshnessRaw);
    if (registryUnavailable) {
      return { state: 'Unavailable', label: 'Unavailable', detail: 'Registry state is unavailable and policy blocks calls through this route.', freshness: freshnessLabel, source: source, noteTitle: 'Route unavailable', noteDetail: 'The target registry marks this route unavailable; no call should be attempted until the registry changes.' };
    }
    if (stale) {
      return { state: 'Stale', label: 'Stale route evidence', detail: 'The latest route evidence is stale; current callability is unknown until telemetry refreshes.', freshness: 'Stale', source: source, noteTitle: 'Route evidence needs refresh', noteDetail: 'The selected route evidence is stale. Current callability is unknown until the route telemetry refreshes.' };
    }
    if (!optedIn) {
      return { state: 'Observation only', label: 'Live readiness unknown', detail: 'Registry state: ' + registry + '. ' + observation + 'No active probe is configured. This view combines registry state with the latest inference observation.', freshness: freshnessLabel, source: source, noteTitle: 'Inference observation only', noteDetail: 'No active probe is configured. Registry state and the latest inference observation do not establish live readiness.' };
    }
    if (freshnessKnown && probeUnavailable) {
      return { state: 'Unavailable', label: 'Unavailable', detail: 'The opted-in route probe reports unavailable. Current calls should not be attempted.', freshness: freshnessLabel, source: source, noteTitle: 'Probe reports unavailable', noteDetail: 'Fresh route evidence reports the opted-in probe as unavailable; this route is not currently callable.' };
    }
    if (probeUnknown || !freshnessKnown) {
      return { state: 'Observation only', label: 'Live readiness unknown', detail: 'Registry state: ' + registry + '. ' + observation + 'The opted-in probe has unknown or unavailable status/freshness, so no active readiness claim is made.', freshness: freshnessLabel, source: source, noteTitle: 'Probe evidence incomplete', noteDetail: 'An opted-in probe is present, but its status or freshness is unknown. Live readiness remains unconfirmed.' };
    }
    var probeLabel = probeReady ? 'Ready' : humanStatus(probeState);
    var until = Number.isFinite(freshUntilMs) ? ' through ' + formatDate(route.freshUntil) : '';
    return { state: probeLabel, label: probeLabel, detail: 'Opted-in route probe reports ' + probeLabel + '. ' + observation + 'Registry state: ' + registry + '.', freshness: freshnessLabel, source: source, noteTitle: 'Fresh probe evidence', noteDetail: 'The opted-in route probe reports ' + probeLabel + '; evidence is ' + freshnessLabel + until + '. This is selected-route evidence, not a capacity guarantee.' };
  }

  function normalizeMembership(value) {
    var item = value || {};
    var organization = firstValue(item, ['organization.name', 'organisation.name', 'organization', 'organisation', 'org_name']);
    var project = firstValue(item, ['project.name', 'project', 'project_slug']);
    var environment = firstValue(item, ['environment.name', 'environment', 'environment_slug']);
    var id = firstValue(item, ['id', 'membership_id', 'slug', 'key']);
    var label = firstValue(item, ['label', 'display_name', 'name']);
    if (!label) label = [organization, project, environment].filter(Boolean).join(' / ');
    return {
      id: stringValue(id, ''),
      label: stringValue(label, 'Membership/session'),
      organization: stringValue(organization, 'Organisation unavailable'),
      project: stringValue(project, 'Project unavailable'),
      environment: stringValue(environment, 'Environment unavailable'),
      current: item.current === true || item.active === true || item.selected === true
    };
  }

  function normalizeMe(payload) {
    var root = unwrap(payload);
    var user = firstValue(root, ['user', 'identity']) || {};
    var context = firstValue(root, ['context', 'current_context', 'scope']) || {};
    var account = firstValue(root, ['account', 'organization', 'organisation']) || {};
    var membership = firstValue(root, ['membership', 'active_membership', 'current_membership']) || {};
    var organization = firstValue(context, ['organization.name', 'organisation.name', 'organization', 'organisation']) || firstValue(root, ['organisation_name', 'organization_name']) || firstValue(account, ['name']);
    var project = firstValue(context, ['project.name', 'project', 'project_slug']) || firstValue(root, ['project_name', 'project.name', 'project', 'project_slug']);
    var environment = firstValue(context, ['environment.name', 'environment', 'environment_slug']) || firstValue(root, ['environment_name', 'environment.name', 'environment', 'environment_slug']);
    var memberships = arrayValue(firstValue(root, ['memberships', 'membership_options', 'available_memberships', 'contexts'])).map(normalizeMembership).filter(function (item) { return item.id || item.label; });
    if (!memberships.length && (membership.id || membership.membership_id || membership.name)) memberships = [normalizeMembership(membership)];
    var currentId = firstValue(context, ['membership_id', 'id']) || firstValue(root, ['membership_id', 'current_membership_id']) || firstValue(membership, ['id', 'membership_id']);
    var current = memberships.filter(function (item) { return item.id === String(currentId || '') || item.current; })[0] || memberships[0] || null;
    if (current) {
      organization = organization || current.organization;
      project = project || current.project;
      environment = environment || current.environment;
    }
    var permissions = firstValue(root, ['permissions', 'access.permissions']) || {};
    return {
      raw: root,
      organization: stringValue(organization, 'Client organisation'),
      project: stringValue(project, 'Project'),
      environment: stringValue(environment, 'Environment'),
      projectEnvironment: [project, environment].filter(Boolean).join(' / ') || 'Project / environment',
      detail: stringValue(firstValue(root, ['scope_detail', 'context_detail', 'detail']), 'This membership/session is scoped by the server to one project/environment.'),
      username: stringValue(firstValue(user, ['username', 'display_name', 'displayName', 'name']) || firstValue(root, ['username']), 'Portal user'),
      role: stringValue(firstValue(context, ['role']) || firstValue(root, ['context.role', 'role', 'membership.role', 'access.role']), 'Portal member'),
      memberships: memberships,
      currentMembershipId: current ? current.id : String(currentId || ''),
      permissions: permissions,
      csrfToken: stringValue(firstValue(root, ['csrf_token', 'csrfToken', 'csrf']), ''),
      gatewayBaseUrl: normalizeGateway(firstValue(root, ['gateway_base_url', 'public_gateway_url', 'gateway.base_url', 'gateway.public_url'])),
      allowedScopes: arrayValue(firstValue(root, ['allowed_scopes', 'key_policy.allowed_scopes', 'access.allowed_scopes'])),
      keyPolicy: firstValue(root, ['key_policy', 'access.key_policy']) || {}
    };
  }

  function normalizeExportMeta(value) {
    var item = value && typeof value === 'object' ? value : {};
    return {
      available: item.available === true,
      formats: arrayValue(firstValue(item, ['formats'])).map(function (format) { return String(format).toLowerCase(); }),
      scope: stringValue(firstValue(item, ['scope']), '')
    };
  }

  function normalizeRoute(value) {
    var item = value || {};
    var servicePlan = firstValue(item, ['service_plan', 'servicePlan', 'plan']) || {};
    var evidenced = executionEvidencePresent(item, servicePlan);
    var executionRaw = firstValue(item, ['execution_class', 'executionClass', 'execution', 'service_plan.execution_class']) || firstValue(servicePlan, ['execution_class', 'execution']);
    var execution = humanExecution(executionRaw, evidenced);
    var customerLabel = firstValue(item, ['customer_execution_label', 'customerExecutionLabel']) || firstValue(servicePlan, ['customer_execution_label', 'customerExecutionLabel']);
    if (evidenced && customerLabel) execution = String(customerLabel);
    var capacity = humanCapacity(firstValue(item, ['capacity_mode', 'capacityMode', 'capacity', 'service_plan.capacity_mode']) || firstValue(servicePlan, ['capacity_mode', 'capacity']), evidenced);
    var registryStatus = stringValue(firstValue(item, ['registry_status', 'registryStatus', 'state', 'status']), 'unknown');
    var latestInferenceStatus = stringValue(firstValue(item, ['latest_inference_status', 'latestInferenceStatus']), 'unknown');
    var probeEnabled = firstValue(item, ['probe_enabled', 'probeEnabled']);
    var probeRaw = firstValue(item, ['probe_status', 'probeStatus']) || {};
    var probeStatus = typeof probeRaw === 'object' ? stringValue(firstValue(probeRaw, ['state', 'status', 'value']), 'unknown') : stringValue(probeRaw, 'unknown');
    var observedAt = firstValue(item, ['observed_at', 'observedAt', 'probe_status.observed_at', 'probeStatus.observedAt']);
    var freshness = stringValue(firstValue(item, ['freshness', 'probe_status.freshness', 'probeStatus.freshness']), 'Unknown');
    var latestInferenceAt = firstValue(item, ['latest_inference_at', 'latestInferenceAt']);
    var latestInferenceSuccess = /^(success|succeeded|ok|complete|completed)$/i.test(String(latestInferenceStatus));
    return {
      raw: item,
      modelAlias: stringValue(firstValue(item, ['model_alias', 'modelAlias', 'alias']), null),
      state: registryStatus,
      registryStatus: registryStatus,
      latestInferenceStatus: latestInferenceStatus,
      statusDetail: stringValue(firstValue(item, ['status_detail', 'statusDetail', 'detail']), 'No route status detail was supplied.'),
      attentionTitle: stringValue(firstValue(item, ['attention.title', 'attention_title']), ''),
      attentionDetail: stringValue(firstValue(item, ['attention.detail', 'attention_detail']), ''),
      executionClass: execution,
      capacityMode: capacity,
      servicePlan: servicePlan,
      lastSuccessAt: firstValue(item, ['last_success_at', 'lastSuccessAt']) || (latestInferenceSuccess ? latestInferenceAt : null),
      latestInferenceAt: latestInferenceAt,
      lastObservationAt: observedAt || latestInferenceAt,
      observedAt: observedAt,
      freshUntil: firstValue(item, ['fresh_until', 'freshUntil', 'probe_status.fresh_until', 'probeStatus.freshUntil']),
      freshness: freshness,
      probeEnabled: probeEnabled,
      probeStatus: probeStatus,
      boundaryDetail: stringValue(firstValue(item, ['boundary_detail', 'service_plan.detail', 'service_plan.description', 'execution_detail']), ''),
      endpointPath: stringValue(firstValue(item, ['endpoint_path', 'endpointPath']), 'POST /v1/chat/completions')
    };
  }

  function normalizeDashboard(payload, me) {
    var root = unwrap(payload);
    var rawRoutes = arrayValue(firstValue(root, ['routes', 'route_registry', 'routeRegistry']));
    var rawRoute = firstValue(root, ['route']);
    if (!rawRoutes.length && Array.isArray(rawRoute)) rawRoutes = rawRoute;
    if (!rawRoutes.length && rawRoute && typeof rawRoute === 'object') rawRoutes = [rawRoute];
    var selectedAlias = state.routeSelection || firstValue(root, ['selected_model', 'selected_model_alias', 'model']);
    var explicitAmbiguous = firstValue(root, ['route_ambiguous', 'route_selection_required', 'ambiguous_route']) === true;
    var ambiguous = explicitAmbiguous || (rawRoutes.length > 1 && !selectedAlias);
    var selectedRaw = null;
    if (rawRoutes.length === 1) selectedRaw = rawRoutes[0];
    if (selectedAlias) selectedRaw = rawRoutes.filter(function (item) { return String(firstValue(item, ['model_alias', 'modelAlias', 'alias']) || '') === String(selectedAlias); })[0] || selectedRaw;
    if (ambiguous && !selectedAlias) selectedRaw = null;
    var selectedRoute = selectedRaw ? normalizeRoute(selectedRaw) : null;
    var usage = firstValue(root, ['usage']) || {};
    var source = firstValue(root, ['source']) || {};
    var tokens = firstValue(usage, ['tokens']) || {};
    var tokenMetrics = firstValue(usage, ['token_metrics', 'tokenMetrics']) || {};
    var totalTokenMetric = firstValue(tokenMetrics, ['total']) || {};
    var breakdowns = firstValue(root, ['breakdowns']) || {};
    var scope = firstValue(root, ['scope', 'context']) || {};
    var meData = me || {};
    var serviceAccounts = arrayValue(firstValue(breakdowns, ['service_accounts', 'serviceAccounts']));
    var routes = rawRoutes.map(normalizeRoute);
    var account = firstValue(root, ['account']) || {};
    var projects = arrayValue(firstValue(breakdowns, ['projects']));
    var models = arrayValue(firstValue(breakdowns, ['models']));
    var logical = numberValue(firstValue(usage, ['logical_requests', 'logicalRequests', 'requests']));
    var successful = numberValue(firstValue(usage, ['successful_requests', 'successfulRequests', 'successes']));
    var blocked = numberValue(firstValue(usage, ['blocked_requests', 'blockedRequests']));
    var failed = numberValue(firstValue(usage, ['failed_requests', 'failedRequests', 'error_requests', 'errors']));
    if (failed === null && logical !== null && successful !== null && blocked !== null) failed = Math.max(0, logical - successful - blocked);
    var failedOnlyTokens = successful === 0 && failed !== null && failed > 0 && (logical === null || failed === logical || (blocked !== null && failed + blocked === logical));
    var totalTokens = firstValue(totalTokenMetric, ['value', 'tokens', 'total', 'count']);
    if (totalTokens === null) totalTokens = firstValue(tokens, ['total', 'value', 'total_tokens']);
    var tokenFinality = firstValue(totalTokenMetric, ['finality']);
    if (failedOnlyTokens) {
      totalTokens = null;
      tokenFinality = null;
    }
    var serviceAccount = firstValue(root, ['service_account.name', 'service_account', 'serviceAccount', 'scope.service_account', 'context.service_account']);
    var servicePlan = selectedRoute && selectedRoute.servicePlan ? selectedRoute.servicePlan : firstValue(root, ['service_plan', 'servicePlan']) || {};
    return {
      raw: root,
      account: { name: stringValue(firstValue(account, ['name']) || firstValue(meData, ['organization']), 'Client organisation'), initials: stringValue(firstValue(account, ['initials']), 'CO') },
      scope: {
        organization: stringValue(firstValue(scope, ['organization', 'organisation', 'organization_name', 'organisation_name']) || firstValue(meData, ['organization']), 'Client organisation'),
        project: stringValue(firstValue(scope, ['project', 'project_name', 'project_slug']) || firstValue(meData, ['project']), 'Project'),
        environment: stringValue(firstValue(scope, ['environment', 'environment_name', 'environment_slug']) || firstValue(meData, ['environment']), 'Environment')
      },
      source: {
        kind: stringValue(firstValue(source, ['kind', 'type']), state.live ? 'Authenticated usage ledger' : FALLBACK.source.kind),
        label: stringValue(firstValue(source, ['label']), state.live ? 'Connected snapshot' : FALLBACK.source.label),
        detail: stringValue(firstValue(source, ['detail']), state.live ? 'Source detail was not supplied.' : FALLBACK.source.detail),
        asOf: firstValue(source, ['as_of', 'asOf']),
        freshness: stringValue(firstValue(source, ['freshness', 'status']), state.live ? 'Unknown' : FALLBACK.source.freshness),
        finality: stringValue(firstValue(source, ['finality', 'token_finality']), state.live ? 'Unknown' : FALLBACK.source.finality)
      },
      route: selectedRoute || (state.live ? null : FALLBACK.route),
      routes: routes,
      ambiguous: ambiguous,
      usage: {
        logicalRequests: logical,
        successfulRequests: successful,
        failedRequests: failed,
        blockedRequests: blocked,
        successRate: numberValue(firstValue(usage, ['success_rate', 'successRate'])),
        errorRate: numberValue(firstValue(usage, ['error_rate', 'errorRate'])),
        p95LatencyMs: numberValue(firstValue(usage, ['p95_latency_ms', 'p95LatencyMs'])),
        throughput: firstValue(usage, ['throughput_rps', 'throughputRps', 'throughput', 'requests_per_second', 'requestsPerSecond']),
        peakConcurrency: firstValue(usage, ['peak_concurrency', 'peakConcurrency']),
        tokens: {
          input: failedOnlyTokens ? null : numberValue(firstValue(tokens, ['input', 'input_tokens'])), output: failedOnlyTokens ? null : numberValue(firstValue(tokens, ['output', 'output_tokens'])), cached: failedOnlyTokens ? null : numberValue(firstValue(tokens, ['cached', 'cached_tokens'])), reasoning: failedOnlyTokens ? null : numberValue(firstValue(tokens, ['reasoning', 'reasoning_tokens'])), total: numberValue(totalTokens)
        },
        tokenFinality: tokenFinality
      },
      allocation: normalizeAllocation(firstValue(usage, ['allowance', 'allocation']) || firstValue(root, ['allowance', 'allocation']), state.live, servicePlan),
      period: { label: periodLabel(root), from: firstValue(root, ['period.from', 'period.start', 'period.from_date']), to: firstValue(root, ['period.to', 'period.end', 'period.to_date']), timezone: stringValue(firstValue(root, ['period.timezone', 'timezone']), 'Timezone not supplied') },
      trend: { unit: stringValue(firstValue(root, ['trend.unit', 'trend_unit']), 'Requests per period'), points: arrayValue(firstValue(root, ['trend.points', 'trend'])).map(normalizeTrendPoint) },
      breakdowns: { projects: projects, models: models, serviceAccounts: serviceAccounts },
      requests: arrayValue(firstValue(root, ['recent_requests', 'recentRequests', 'requests'])),
      exportMeta: normalizeExportMeta(firstValue(root, ['export'])),
      gatewayBaseUrl: normalizeGateway(firstValue(root, ['gateway_base_url', 'public_gateway_url'])) || (meData && meData.gatewayBaseUrl) || null,
      attribution: { serviceAccount: stringValue(serviceAccount, 'Unavailable in this snapshot') }
    };
  }

  function blankDashboard(message) {
    var blank = JSON.parse(JSON.stringify(FALLBACK));
    blank.source = { kind: 'Unavailable', label: 'Usage unavailable', detail: message || 'The connected usage source did not respond.', asOf: null, freshness: 'Unavailable', finality: 'Unknown' };
    blank.route = null;
    blank.routes = [];
    blank.usage.logicalRequests = null;
    blank.allocation = normalizeAllocation(null, true, null);
    blank.gatewayBaseUrl = null;
    return blank;
  }

  function normalizeAllocation(value, live, servicePlan) {
    var item = value && typeof value === 'object' ? value : {};
    var plan = servicePlan && typeof servicePlan === 'object' ? servicePlan : {};
    var sources = [item, plan];
    function fromSources(candidates) {
      for (var i = 0; i < sources.length; i += 1) {
        var found = firstValue(sources[i], candidates);
        if (found !== null) return found;
      }
      return null;
    }
    var allowanceObject = fromSources(['allowance', 'allowance_context', 'shared_allowance', 'allocation', 'capacity_allocation']);
    if (allowanceObject && typeof allowanceObject === 'object') sources.unshift(allowanceObject);
    var shared = fromSources(['shared', 'shared_allowance', 'sharedAllowance', 'allowance', 'limit', 'value', 'amount']);
    var dedicated = fromSources(['dedicated', 'dedicated_allocation', 'dedicatedAllocation', 'dedicated_value']);
    if (shared === null && allowanceObject && typeof allowanceObject === 'object' &&
        (Object.prototype.hasOwnProperty.call(allowanceObject, 'logical_requests') ||
         Object.prototype.hasOwnProperty.call(allowanceObject, 'provider_reported_tokens'))) {
      shared = allowanceObject;
    }
    if (dedicated === null && allowanceObject && typeof allowanceObject === 'object' &&
        (Object.prototype.hasOwnProperty.call(allowanceObject, 'resource_class') ||
         Object.prototype.hasOwnProperty.call(allowanceObject, 'accelerator_count'))) {
      dedicated = allowanceObject;
    }
    return {
      shared: live ? formatSharedAllowance(shared, 'Unknown — no contractual value supplied') : FALLBACK.allocation.shared,
      dedicated: live ? formatDedicatedAllocation(dedicated, 'Unknown — no dedicated allocation evidenced') : FALLBACK.allocation.dedicated,
      source: stringValue(fromSources(['source', 'source_label']), live ? 'Contract source unavailable' : FALLBACK.allocation.source),
      finality: stringValue(fromSources(['finality']), live ? 'Unknown' : FALLBACK.allocation.finality),
      contextDetail: live ? stringValue(fromSources(['detail', 'context_detail', 'description']), 'Allowance and allocation values appear only when an authoritative contract source supplies them.') : FALLBACK.allocation.contextDetail
    };
  }

  function formatPlanEntry(label, value) {
    if (value === undefined || value === null || value === '') return null;
    if (typeof value === 'object') {
      var amount = firstValue(value, ['value']);
      if (amount === null) return null;
      var unit = firstValue(value, ['unit', 'units']);
      var period = firstValue(value, ['period', 'window']);
      var suffix = [unit, period ? '/ ' + String(period) : null].filter(Boolean).join(' ');
      return label + ': ' + String(amount) + (suffix ? ' ' + suffix : '');
    }
    return label + ': ' + String(value);
  }

  function formatSharedAllowance(value, fallback) {
    if (value && typeof value === 'object') {
      var entries = [
        formatPlanEntry('Logical requests', firstValue(value, ['logical_requests', 'logicalRequests', 'logical_request_limit', 'logicalRequestLimit'])),
        formatPlanEntry('Reported tokens', firstValue(value, ['provider_reported_tokens', 'providerReportedTokens', 'reported_token_limit', 'reportedTokenLimit']))
      ].filter(Boolean);
      if (entries.length) return entries.join(' · ');
    }
    return formatContractValue(value, fallback);
  }

  function formatDedicatedAllocation(value, fallback) {
    if (value && typeof value === 'object') {
      var entries = [
        firstValue(value, ['resource_class', 'resourceClass']) !== null ? String(firstValue(value, ['resource_class', 'resourceClass'])) : null,
        firstValue(value, ['accelerator_count', 'acceleratorCount']) !== null ? String(firstValue(value, ['accelerator_count', 'acceleratorCount'])) + ' accelerators' : null
      ].filter(Boolean);
      if (entries.length) return entries.join(' · ');
    }
    return formatContractValue(value, fallback);
  }

  function formatContractValue(value, fallback) {
    if (value === undefined || value === null || value === '') return fallback;
    if (typeof value === 'object') {
      var amount = firstValue(value, ['value', 'amount', 'limit', 'quantity']);
      var unit = firstValue(value, ['unit', 'units']);
      if (amount !== null) return String(amount) + (unit ? ' ' + String(unit) : '');
      return fallback;
    }
    return String(value);
  }

  function normalizeTrendPoint(value) {
    var item = value || {};
    var tokenValue = firstValue(item, ['tokens.value', 'tokens', 'total_tokens']);
    var logicalRequests = numberValue(firstValue(item, ['logical_requests', 'requests']));
    var successfulRequests = numberValue(firstValue(item, ['successful_requests', 'successfulRequests', 'successes']));
    var successRate = logicalRequests === 0 && successfulRequests === 0 ? 0 : (logicalRequests !== null && logicalRequests > 0 && successfulRequests !== null ? Math.max(0, Math.min(1, successfulRequests / logicalRequests)) : null);
    return {
      label: stringValue(firstValue(item, ['label', 'period', 'bucket_start']), 'Period'),
      requests: logicalRequests,
      tokens: numberValue(tokenValue),
      successRate: successRate,
      p95LatencyMs: numberValue(firstValue(item, ['p95_latency_ms', 'p95LatencyMs']))
    };
  }

  function humanMode(value) {
    var key = String(value || '').toLowerCase().replace(/[\s-]+/g, '_');
    if (key === 'shared' || key === 'shared_pilot' || key === 'shared_evaluation' || key === 'shared_subscription' || key === 'paid_shared') return 'Shared';
    if (key === 'dedicated' || key === 'private' || key === 'private_compatible' || key === 'dedicated_private') return 'Dedicated';
    return value ? String(value) : 'Unknown';
  }

  function formatFact(value, fallback) {
    if (value === undefined || value === null || value === '') return fallback || 'Unknown';
    if (Array.isArray(value)) return value.length ? value.map(String).join(', ') : (fallback || 'Unknown');
    if (typeof value === 'object') {
      var amount = firstValue(value, ['value', 'amount', 'count', 'quantity']);
      var unit = firstValue(value, ['unit', 'units']);
      var period = firstValue(value, ['period', 'window']);
      if (amount !== null) return String(amount) + (unit ? ' ' + String(unit) : '') + (period ? ' / ' + String(period) : '');
      return fallback || 'Unknown';
    }
    return String(value);
  }

  function formatMoneyMinor(amount, currency) {
    var minor = numberValue(amount);
    var code = String(currency || '').trim().toUpperCase();
    if (minor === null || !code) return null;
    try {
      return new Intl.NumberFormat(undefined, { style: 'currency', currency: code }).format(minor / 100);
    } catch (error) {
      return code + ' ' + (minor / 100).toFixed(2);
    }
  }

  function formatPrice(value, fallback) {
    if (value === undefined || value === null || value === '') return fallback || 'Unknown';
    if (typeof value !== 'object') return String(value);
    var currency = firstValue(value, ['currency']);
    var recurring = formatMoneyMinor(firstValue(value, ['recurring_amount_minor', 'amount_minor']), currency);
    var setup = formatMoneyMinor(firstValue(value, ['setup_amount_minor']), currency);
    var period = firstValue(value, ['billing_period', 'period']);
    var finality = firstValue(value, ['finality', 'price_finality']);
    var source = firstValue(value, ['source']);
    var parts = [];
    if (recurring) parts.push(recurring + (period ? ' / ' + String(period) : ''));
    if (setup && numberValue(firstValue(value, ['setup_amount_minor'])) !== 0) parts.push(setup + ' setup');
    if (finality) parts.push(humanStatus(finality));
    if (source) parts.push(String(source));
    return parts.length ? parts.join(' · ') : formatFact(value, fallback || 'Unknown');
  }

  function formatCapabilities(value, fallback) {
    if (Array.isArray(value)) return value.length ? value.map(String).join(', ') : (fallback || 'Unknown');
    if (value && typeof value === 'object') {
      var entries = Object.keys(value).filter(function (key) { return value[key] !== false && value[key] !== null && value[key] !== ''; }).map(function (key) { return key.replace(/[_-]+/g, ' '); });
      if (entries.length) return entries.join(', ');
    }
    return formatFact(value, fallback);
  }

  function normalizeProfile(value, model) {
    var item = value || {};
    var offer = firstValue(item, ['offer', 'commercial_offer', 'pricing']) || item;
    var profile = firstValue(item, ['profile', 'deployment_profile']) || item;
    var kind = firstValue(offer, ['kind', 'offer_kind']);
    var executionRaw = firstValue(profile, ['execution_class', 'executionClass', 'execution']);
    var modeRaw = firstValue(item, ['mode', 'service_mode', 'deployment_mode', 'execution_mode', 'type']) || (/dedicated|private/i.test(String(kind || executionRaw || '')) ? 'dedicated' : 'shared');
    var minimumUnits = numberValue(firstValue(profile, ['minimum_capacity_units', 'minimum_units']));
    var maximumUnits = numberValue(firstValue(profile, ['maximum_capacity_units', 'maximum_units']));
    var accelerator = firstValue(profile, ['accelerator_class']);
    var perUnit = numberValue(firstValue(profile, ['accelerators_per_unit']));
    var capacity = firstValue(item, ['capacity', 'capacity_context', 'allocation', 'resource', 'resource_class']);
    if (capacity === null) capacity = [minimumUnits !== null ? minimumUnits + (maximumUnits !== null && maximumUnits !== minimumUnits ? '–' + maximumUnits : '') + ' unit' + (maximumUnits === 1 ? '' : 's') : null, accelerator ? String(accelerator) : null, perUnit !== null ? perUnit + ' accelerator' + (perUnit === 1 ? '' : 's') + ' / unit' : null].filter(Boolean).join(' · ');
    var availability = firstValue(offer, ['availability', 'availability_state', 'status', 'lifecycle']);
    var price = firstValue(offer, ['price', 'cost', 'estimate', 'price_range', 'cost_range']);
    var payment = firstValue(offer, ['payment']) || {};
    var evidenced = firstValue(profile, ['evidence_provided']) === true || executionEvidencePresent(profile, offer);
    var offerCode = stringValue(firstValue(offer, ['code', 'offer_code', 'id']), '');
    var profileCode = stringValue(firstValue(profile, ['code', 'profile_code', 'id']), '');
    return {
      raw: item,
      id: offerCode || profileCode,
      offerCode: offerCode,
      profileCode: profileCode,
      name: stringValue(firstValue(offer, ['name', 'display_name', 'label']), stringValue(firstValue(profile, ['name', 'display_name', 'profile_name']), 'Profile unavailable')),
      mode: humanMode(modeRaw),
      modeRaw: modeRaw,
      executionClass: humanExecution(executionRaw, evidenced) || 'Unknown',
      capacity: formatFact(capacity, 'Unknown'),
      capacityRaw: capacity,
      availability: humanStatus(availability),
      availabilityRaw: availability,
      commercial: stringValue(kind, formatFact(firstValue(offer, ['commercial_state', 'commercial_status', 'billing_state', 'offer_state', 'state', 'status']), 'Unknown')).replace(/_/g, ' '),
      price: price === null ? 'Unknown' : formatPrice(price, 'Unknown'),
      assumptions: stringValue(firstValue(offer, ['assumptions', 'estimate_assumptions', 'description', 'detail']) || firstValue(payment, ['detail']), ''),
      eligible: firstValue(offer, ['eligible', 'is_eligible']) !== false,
      source: stringValue(firstValue(offer, ['source.label', 'source', 'evidence_source']) || firstValue(profile, ['source']), 'Registry'),
      freshness: stringValue(firstValue(offer, ['published_at', 'freshness', 'source.freshness']), 'Unknown'),
      minimumCapacityUnits: minimumUnits,
      maximumCapacityUnits: maximumUnits,
      modelSlug: model && model.slug ? model.slug : stringValue(firstValue(item, ['model_slug']), '')
    };
  }

  function normalizeModel(value) {
    var item = firstValue(value || {}, ['model']) || value || {};
    var rawProfiles = arrayValue(firstValue(item, ['offers', 'profiles', 'deployment_profiles', 'eligible_profiles']));
    var capabilities = firstValue(item, ['capabilities', 'capability_facts']) || {};
    var modalities = firstValue(item, ['modalities', 'input_modalities', 'capabilities.modalities']) || firstValue(capabilities, ['modalities', 'input']);
    var release = firstValue(item, ['recommended_release', 'release']) || {};
    var context = firstValue(item, ['context_window', 'context_limit', 'max_context_tokens', 'capabilities.context_window']) || firstValue(release, ['context_window_tokens']) || firstValue(capabilities, ['context_window', 'context_limit', 'max_tokens']);
    var slug = stringValue(firstValue(item, ['slug', 'model_slug', 'id', 'name']), '');
    return {
      raw: item,
      id: stringValue(firstValue(item, ['id', 'model_id']), slug),
      slug: slug,
      endpointAlias: stringValue(firstValue(item, ['endpoint_alias', 'model_alias', 'alias']), slug),
      name: stringValue(firstValue(item, ['display_name', 'name', 'family', 'model_name']), slug || 'Model unavailable'),
      release: stringValue(firstValue(release, ['version', 'release_name', 'revision']) || firstValue(item, ['version', 'release_name', 'revision']), 'Release unavailable'),
      lifecycle: humanStatus(firstValue(item, ['lifecycle', 'lifecycle_state', 'status'])),
      capabilities: capabilities,
      capabilitiesText: formatCapabilities(firstValue(item, ['capabilities', 'capability_summary']) || modalities, 'Unknown'),
      modalities: formatFact(modalities, 'Unknown'),
      context: formatFact(context, 'Unknown'),
      licence: stringValue(firstValue(item, ['licence', 'license']) || firstValue(release, ['licence_name', 'license_name']), 'Unknown'),
      support: stringValue(firstValue(item, ['support', 'support_level']) || firstValue(release, ['support_status']), 'Unknown'),
      source: stringValue(firstValue(item, ['source.label', 'source', 'catalogue_source']) || firstValue(release, ['source']), 'Catalogue'),
      freshness: stringValue(firstValue(item, ['freshness', 'source.freshness', 'as_of']) || firstValue(release, ['published_at']), 'Unknown'),
      availability: stringValue(firstValue(item, ['availability', 'availability_state']), 'Unknown'),
      profiles: rawProfiles.map(function (profile) { return normalizeProfile(profile, { slug: slug }); }),
      summary: stringValue(firstValue(item, ['summary', 'description', 'detail']), 'No model description supplied by the registry.')
    };
  }

  function normalizeCatalogue(payload) {
    var root = Array.isArray(payload) ? { models: payload } : unwrap(payload);
    var models = arrayValue(firstValue(root, ['models', 'entries', 'items', 'catalogue.models'])).map(normalizeModel);
    return {
      raw: root,
      unavailable: false,
      source: stringValue(firstValue(root, ['source.label', 'source', 'catalogue_source']), 'Connected model catalogue'),
      detail: stringValue(firstValue(root, ['source.detail', 'detail']), 'Model facts are supplied by the connected service registry.'),
      models: models,
      asOf: firstValue(root, ['as_of', 'source.as_of'])
    };
  }

  function unavailableCatalogue(message) {
    return { raw: {}, unavailable: true, source: 'Catalogue unavailable', detail: message || 'The model catalogue could not be loaded. No model availability claim is made.', models: [] };
  }

  function endpointModeRaw(item) {
    return firstValue(item, ['mode', 'service_mode', 'deployment_mode', 'endpoint_mode', 'type']);
  }

  function normalizeEndpoint(value) {
    var item = firstValue(value || {}, ['endpoint']) || value || {};
    var servicePlan = firstValue(item, ['service_plan', 'servicePlan', 'plan']) || {};
    var runtimeRail = firstValue(item, ['runtime']) || {};
    var configurationRail = firstValue(item, ['configuration']) || {};
    var commercialRail = firstValue(item, ['commercial']) || {};
    var executionEvidenced = firstValue(item, ['evidence_provided', 'execution_evidence_provided']) === true || executionEvidencePresent(item, servicePlan);
    var routeInput = {};
    Object.keys(item).forEach(function (key) { routeInput[key] = item[key]; });
    routeInput.registry_status = firstValue(item, ['registry_status', 'registryStatus', 'route_status', 'status']) || (item.route_bound === true ? 'bound' : item.route_bound === false ? 'unbound' : null);
    routeInput.probe_enabled = firstValue(item, ['probe_enabled', 'probeEnabled']);
    routeInput.probe_status = firstValue(item, ['probe_status', 'probeStatus', 'readiness_probe']);
    routeInput.freshness = firstValue(item, ['freshness', 'probe_freshness', 'readiness_freshness']);
    routeInput.fresh_until = firstValue(item, ['fresh_until', 'freshUntil']);
    routeInput.latest_inference_status = firstValue(item, ['latest_inference_status', 'latestInferenceStatus', 'latest_observation_status']);
    routeInput.observed_at = firstValue(item, ['observed_at', 'observedAt', 'latest_observed_at']);
    var route = normalizeRoute(routeInput);
    var modeRaw = endpointModeRaw(item);
    var allowance = normalizeAllocation(firstValue(item, ['allowance', 'allocation', 'capacity']) || {}, state.live, servicePlan);
    var model = firstValue(item, ['model', 'model_ref']) || {};
    var safeGateway = normalizeGateway(firstValue(item, ['gateway_base_url', 'public_gateway_url', 'endpoint_base_url', 'safe_gateway_url']));
    return {
      raw: item,
      id: stringValue(firstValue(item, ['id', 'endpoint_id', 'slug']), ''),
      alias: stringValue(firstValue(item, ['alias', 'model_alias', 'endpoint_alias', 'name']), 'Alias unavailable'),
      modelSlug: stringValue(firstValue(item, ['model_slug', 'model_id']) || firstValue(model, ['slug', 'id']), ''),
      modelName: stringValue(firstValue(item, ['model_name', 'model_display_name']) || firstValue(model, ['name', 'display_name', 'family']), 'Model unavailable'),
      release: stringValue(firstValue(item, ['release', 'model_release', 'model_version', 'version']) || firstValue(model, ['release', 'version']), 'Release unavailable'),
      mode: humanMode(modeRaw),
      modeRaw: modeRaw,
      environment: stringValue(firstValue(item, ['environment', 'environment_name', 'environment_slug']) || (state.me && state.me.environment), 'Environment unavailable'),
      project: stringValue(firstValue(item, ['project', 'project_name', 'project_slug']) || (state.me && state.me.project), 'Project unavailable'),
      executionClass: humanExecution(firstValue(item, ['execution_class', 'executionClass', 'execution']) || firstValue(servicePlan, ['execution_class']), executionEvidenced) || 'Unknown',
      lifecycle: humanStatus(firstValue(item, ['lifecycle', 'lifecycle_state', 'status']) || firstValue(configurationRail, ['state'])),
      runtimeStatus: humanStatus(firstValue(item, ['runtime_status', 'readiness', 'endpoint_status', 'status']) || firstValue(runtimeRail, ['state'])),
      commercial: humanStatus(firstValue(item, ['commercial_state', 'commercial_status', 'payment_state', 'billing_state', 'quote_state']) || firstValue(commercialRail, ['state'])),
      commercialRaw: firstValue(item, ['commercial_state', 'commercial_status', 'payment_state', 'billing_state', 'quote_state']) || firstValue(commercialRail, ['state']),
      route: route,
      evidence: routeEvidence(route),
      allowance: allowance,
      allowanceRaw: firstValue(item, ['allowance', 'allocation']),
      capacity: firstValue(item, ['capacity_units']) !== null ? String(firstValue(item, ['capacity_units'])) + ' configured capacity unit' + (Number(firstValue(item, ['capacity_units'])) === 1 ? '' : 's') : formatFact(firstValue(item, ['capacity', 'capacity_context', 'allocation']) || firstValue(servicePlan, ['capacity', 'allocation']), 'Unknown'),
      endpointPath: stringValue(firstValue(item, ['endpoint_path', 'endpointPath', 'api_path']), '/v1/chat/completions'),
      gatewayBaseUrl: safeGateway,
      lastObservedAt: firstValue(item, ['observed_at', 'observedAt', 'latest_observed_at', 'last_observed_at']) || route.lastObservationAt,
      usageHref: queryPath('/app/usage', { model: firstValue(item, ['model_alias', 'alias', 'endpoint_alias']) }),
      source: stringValue(firstValue(item, ['source.label', 'source', 'evidence_source']), 'Alzette endpoint registry'),
      freshness: stringValue(firstValue(item, ['freshness', 'source.freshness']), route.freshness || 'Unknown'),
      servicePlan: servicePlan,
      capacityUnits: numberValue(firstValue(item, ['capacity_units'])),
      action: stringValue(firstValue(item, ['next_action', 'action']), '')
    };
  }

  function normalizeEndpoints(payload) {
    var root = Array.isArray(payload) ? { endpoints: payload } : unwrap(payload);
    var endpoints = arrayValue(firstValue(root, ['endpoints', 'items', 'routes'])).map(normalizeEndpoint);
    return { raw: root, unavailable: false, source: stringValue(firstValue(root, ['source.label', 'source']), 'Connected endpoint inventory'), detail: stringValue(firstValue(root, ['source.detail', 'detail']), 'Endpoint facts are supplied by the connected endpoint registry.'), endpoints: endpoints, asOf: firstValue(root, ['as_of', 'source.as_of']) };
  }

  function unavailableEndpoints(message) {
    return { raw: {}, unavailable: true, source: 'Endpoint inventory unavailable', detail: message || 'The endpoint inventory could not be loaded. No endpoint readiness or commercial claim is made.', endpoints: [] };
  }

  function normalizeProgressRail(value, fallback) {
    var item = value && typeof value === 'object' ? value : {};
    var raw = typeof value === 'string' ? value : firstValue(item, ['state', 'status', 'phase', 'value']);
    return { raw: raw, label: humanStatus(raw), detail: stringValue(firstValue(item, ['detail', 'message', 'description']), fallback), state: String(raw || 'unknown').toLowerCase().replace(/[\s-]+/g, '_') };
  }

  function normalizeRequestProgress(payload) {
    var envelope = unwrap(payload);
    var root = firstValue(envelope, ['deployment_request']) || envelope;
    var status = stringValue(firstValue(root, ['status', 'state']), 'unknown').toLowerCase().replace(/[\s-]+/g, '_');
    var quoteId = stringValue(firstValue(root, ['quote_id', 'deployment_quote_id', 'quote.id']), '');
    var paymentRequirementId = stringValue(firstValue(root, ['payment_requirement_id', 'payment_requirement.id']), '');
    var configuration = normalizeProgressRail(firstValue(root, ['configuration', 'configuration_status']) || 'submitted', 'The endpoint configuration was submitted to this scoped deployment request.');
    var commercialFallback = quoteId ? (status === 'accepted' || status === 'approved' || status === 'allocating' || status === 'deploying' || status === 'validating' || status === 'ready' ? 'accepted' : 'quoted') : (status === 'submitted' ? 'quote_pending' : 'unknown');
    var commercial = normalizeProgressRail(firstValue(root, ['commercial', 'commercial_status', 'quote']) || commercialFallback, quoteId ? 'A versioned quote is attached to this request.' : 'No versioned quote is attached yet.');
    var payment = normalizeProgressRail(firstValue(root, ['payment', 'payment_status', 'payment_requirement']) || (paymentRequirementId ? 'action_required' : 'unknown'), paymentRequirementId ? 'A scoped payment requirement is attached; confirmation remains server-authoritative.' : 'No payment requirement is attached to this request.');
    var infrastructure = normalizeProgressRail(firstValue(root, ['infrastructure', 'deployment', 'runtime', 'infrastructure_status']) || status, 'Infrastructure status is reported independently from commercial and payment state.');
    var workload = firstValue(root, ['workload', 'sizing_intent']) || {};
    return {
      raw: root,
      id: stringValue(firstValue(root, ['id', 'request_id', 'deployment_request_id']), 'Request reference unavailable'),
      model: stringValue(firstValue(root, ['model_name', 'model_slug', 'model']), 'Model unavailable'),
      alias: stringValue(firstValue(root, ['alias', 'model_alias', 'endpoint_alias']), 'Alias unavailable'),
      configuration: configuration,
      commercial: commercial,
      payment: payment,
      infrastructure: infrastructure,
      quoteId: quoteId,
      paymentRequirementId: paymentRequirementId,
      endpointId: stringValue(firstValue(root, ['endpoint_id', 'endpoint.id']), ''),
      kind: stringValue(firstValue(root, ['kind', 'request_kind']), 'new_endpoint'),
      currentCapacityUnits: numberValue(firstValue(root, ['current_capacity_units'])),
      requestedCapacityUnits: numberValue(firstValue(root, ['requested_capacity_units', 'capacity_units'])),
      workload: workload,
      nextAction: stringValue(firstValue(root, ['next_action', 'action', 'recommended_action']), 'Follow the state above; no next action was supplied.'),
      source: stringValue(firstValue(root, ['source.label', 'source']), 'Deployment request source')
    };
  }

  function normalizeBilling(payload) {
    var root = unwrap(payload);
    var query = new URLSearchParams(window.location.search || '');
    var returnState = query.get('billing_return') || query.get('checkout');
    var stateValue = firstValue(root, ['state', 'billing_state', 'status', 'configuration']) || (returnState === 'cancel' ? 'cancellation' : returnState === 'success' ? 'success_return' : 'not_configured');
    var invoices = arrayValue(firstValue(root, ['invoices', 'billing_documents', 'receipts']));
    return {
      raw: root,
      unavailable: false,
      state: String(stateValue).toLowerCase().replace(/[\s-]+/g, '_'),
      stateLabel: humanStatus(stateValue),
      source: stringValue(firstValue(root, ['source.label', 'source']), 'Connected billing record'),
      detail: stringValue(firstValue(root, ['source.detail', 'detail', 'message']), 'Billing state is supplied by the Alzette billing service.'),
      account: stringValue(firstValue(root, ['account_name', 'customer_name', 'organisation_name', 'organization_name']), 'Alzette account'),
      commercial: stringValue(firstValue(root, ['commercial_state', 'billing_state', 'status']), 'Unknown'),
      taxStatus: stringValue(firstValue(root, ['tax_status', 'legal_status']), 'Unknown'),
      checkout: stringValue(firstValue(root, ['checkout_state', 'payment_state']), returnState === 'cancel' ? 'Checkout cancelled — no payment state changed.' : returnState === 'success' ? 'Payment confirmation pending' : 'Unknown'),
      confirmedAt: firstValue(root, ['confirmed_at', 'as_of', 'updated_at']),
      canManage: firstValue(root, ['can_manage', 'canManage', 'permissions.manage_billing']) === true,
      configured: firstValue(root, ['configured', 'stripe_configured', 'billing_configured']),
      invoices: invoices,
      returnState: returnState
    };
  }

  function unavailableBilling(message) {
    return { raw: {}, unavailable: true, state: 'not_configured', stateLabel: 'Billing not configured', source: 'Billing unavailable', detail: message || 'The billing endpoint did not return a usable snapshot.', account: 'Unknown', commercial: 'Unknown', taxStatus: 'Unknown', checkout: 'Unknown', confirmedAt: null, canManage: false, configured: false, invoices: [] };
  }

  function normalizeAccess(payload) {
    var root = unwrap(payload);
    var permissions = firstValue(root, ['permissions', 'access.permissions']) || {};
    var keyPolicy = firstValue(root, ['key_policy', 'keyPolicy', 'policy']) || {};
    var allowed = arrayValue(firstValue(root, ['allowed_scopes', 'key_policy.allowed_scopes', 'access.allowed_scopes', 'permissions.allowed_scopes']) || firstValue(keyPolicy, ['allowed_scopes'])).map(String);
    var serviceAccounts = arrayValue(firstValue(root, ['service_accounts', 'serviceAccounts', 'accounts'])).map(normalizeServiceAccount);
    var nestedKeys = [];
    serviceAccounts.forEach(function (account) { nestedKeys = nestedKeys.concat(account.keys || []); });
    var topKeys = arrayValue(firstValue(root, ['keys', 'api_keys', 'apiKeys'])).map(function (key) { return normalizeKey(key, null); });
    var keys = topKeys.concat(nestedKeys).filter(function (key, index, list) { return list.findIndex(function (item) { return item.prefix === key.prefix; }) === index; });
    return {
      raw: root,
      unavailable: false,
      source: stringValue(firstValue(root, ['source.label', 'source', 'as_of']), 'Access snapshot'),
      permissions: permissions,
      role: stringValue(firstValue(root, ['role', 'permissions.role']), ''),
      canManage: firstValue(root, ['can_manage', 'canManage']) === true || permissions.can_manage === true,
      allowedScopes: allowed,
      keyPolicy: keyPolicy,
      serviceAccounts: serviceAccounts,
      keys: keys
    };
  }

  function unavailableAccess(message) {
    return {
      raw: {}, unavailable: true,
      source: 'Access metadata unavailable',
      detail: message || 'Access metadata could not be evaluated. No inventory or permission claim is made.',
      permissions: {}, role: '', canManage: false, allowedScopes: [], keyPolicy: {}, serviceAccounts: [], keys: []
    };
  }

  function normalizeServiceAccount(value) {
    var item = value || {};
    return {
      id: stringValue(firstValue(item, ['id', 'service_account_id', 'slug']), ''),
      name: stringValue(firstValue(item, ['name', 'display_name']), 'Unnamed service account'),
      description: stringValue(firstValue(item, ['description']), ''),
      status: stringValue(firstValue(item, ['status']), 'Configured'),
      createdAt: firstValue(item, ['created_at', 'createdAt']),
      keys: arrayValue(firstValue(item, ['keys'])).map(function (key) { return normalizeKey(key, { id: firstValue(item, ['id', 'service_account_id', 'slug']), name: firstValue(item, ['name', 'display_name']) }); })
    };
  }

  function normalizeKey(value, parent) {
    var item = value || {};
    var owner = parent || {};
    return {
      id: stringValue(firstValue(item, ['id', 'key_id']), ''),
      serviceAccountId: stringValue(firstValue(item, ['service_account_id', 'serviceAccountId', 'account_id']) || firstValue(owner, ['id']), ''),
      serviceAccountName: stringValue(firstValue(item, ['service_account_name', 'serviceAccountName', 'account_name']) || firstValue(owner, ['name']), 'Unknown service account'),
      name: stringValue(firstValue(item, ['name', 'key_name']), 'Unnamed key'),
      prefix: stringValue(firstValue(item, ['prefix', 'key_prefix']), 'Unavailable'),
      scopes: arrayValue(firstValue(item, ['scopes'])).map(String),
      createdAt: firstValue(item, ['created_at', 'createdAt']),
      expiresAt: firstValue(item, ['expires_at', 'expiresAt', 'expiry']),
      lastUsedAt: firstValue(item, ['last_used_at', 'lastUsedAt', 'last_used']),
      status: stringValue(firstValue(item, ['status']), 'Unknown')
    };
  }

  function newIdempotencyKey(prefix) {
    var random = window.crypto && typeof window.crypto.randomUUID === 'function' ? window.crypto.randomUUID() : String(Date.now()) + '-' + Math.random().toString(16).slice(2);
    return String(prefix || 'portal').replace(/[^a-z0-9:_-]+/gi, '-').slice(0, 80) + '-' + random;
  }

  function operationKey(name) {
    var operation = String(name || 'portal');
    if (!state.operationKeys[operation]) state.operationKeys[operation] = newIdempotencyKey(operation);
    return state.operationKeys[operation];
  }

  function clearOperationKey(name) {
    if (name) delete state.operationKeys[String(name)];
  }

  function responseErrorCode(result) {
    return stringValue(firstValue(result && result.data || {}, ['error.code', 'code']), '').toLowerCase();
  }

  function responseMessage(result, fallback) {
    return stringValue(firstValue(result && result.data || {}, ['error.message', 'message']), fallback || 'The request could not be completed.');
  }

  function recentAuthenticationRequired(result) {
    return !!result && result.status === 401 && responseErrorCode(result) === 'recent_authentication_required';
  }

  function definitiveResult(result) {
    return !!result && result.status > 0 && result.status !== 408 && result.status !== 425 && result.status !== 429 && result.status < 500;
  }

  async function request(path, options) {
    var config = options || {};
    var method = config.method || 'GET';
    var headers = { Accept: 'application/json' };
    if (config.headers) Object.keys(config.headers).forEach(function (key) { headers[key] = config.headers[key]; });
    if (state.csrfToken && method !== 'GET' && method !== 'HEAD') headers['X-CSRF-Token'] = state.csrfToken;
    if ((config.idempotency || config.idempotencyKey) && method !== 'GET' && method !== 'HEAD') {
      var operationName = config.idempotency ? (config.idempotency === true ? 'portal' : String(config.idempotency)) : '';
      var stableKey = config.idempotencyKey || operationKey(operationName);
      headers['Idempotency-Key'] = stableKey;
    }
    var body = config.body;
    if (body && typeof body === 'object' && !(body instanceof FormData)) {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(body);
    }
    try {
      var response = await fetch(path, { method: method, headers: headers, body: body, credentials: 'same-origin', redirect: config.redirect || 'follow' });
      var raw = await response.text();
      var data = null;
      if (raw) {
        try { data = JSON.parse(raw); } catch (error) { data = { message: raw }; }
      }
      return { ok: response.ok, status: response.status, data: data, headers: response.headers, operationName: operationName || '' };
    } catch (error) {
      return { ok: false, status: 0, data: null, error: error, operationName: operationName || '' };
    }
  }

  async function firstAvailable(list, options) {
    var last = { ok: false, status: 0, data: null };
    for (var i = 0; i < list.length; i += 1) {
      var result = await request(list[i], options);
      last = result;
      if (result.ok || result.status === 401 || result.status === 403 || (result.status >= 500 && result.status !== 501)) return result;
      if (result.status !== 404 && result.status !== 405) return result;
    }
    return last;
  }

  function queryPath(path, params) {
    var entries = Object.keys(params || {}).filter(function (key) { return params[key] !== undefined && params[key] !== null && params[key] !== ''; });
    if (!entries.length) return path;
    return path + '?' + entries.map(function (key) { return encodeURIComponent(key) + '=' + encodeURIComponent(params[key]); }).join('&');
  }

  function dateInputRFC3339(value, endOfDay) {
    if (!value) return '';
    var parts = String(value).match(/^(\d{4})-(\d{2})-(\d{2})$/);
    if (!parts) return '';
    var date = new Date(Date.UTC(Number(parts[1]), Number(parts[2]) - 1, Number(parts[3]), 0, 0, 0, 0));
    if (endOfDay) date.setUTCDate(date.getUTCDate() + 1);
    return Number.isNaN(date.getTime()) ? '' : date.toISOString();
  }

  function renderGlobal(source, status, overrideDetail) {
    var global = q('#global-state');
    setState(global, status);
    var badges = {
      live: 'Live scoped ledger',
      stale: 'Stale scoped ledger',
      partial: 'Partial scoped ledger',
      loading: 'Refreshing data',
      error: 'Data unavailable',
      unavailable: 'Data unavailable',
      fallback: 'Illustrative preview'
    };
    bind('source.badge', badges[status] || source.label || 'Source unavailable');
    bind('source.label', source.label);
    bind('source.detail', overrideDetail || source.detail);
    bind('source.freshness', source.freshness);
    bind('source.finality', source.finality);
    bind('source.kind', source.kind);
    bind('source.asOfText', source.asOf ? formatDate(source.asOf) : 'Not available');
    bindTime('source.asOf', source.asOf);
    if (global) {
      var time = q('time[data-bind="source.asOf"]', global);
      if (time) time.textContent = source.asOf ? 'As of ' + formatDate(source.asOf) : 'As of not available';
    }
  }

  function renderMe(me) {
    var value = me || normalizeMe({});
    bind('scope.organization', value.organization);
    bind('scope.project', value.project);
    bind('scope.environment', value.environment);
    bind('scope.projectEnvironment', value.projectEnvironment);
    bind('scope.detail', value.detail);
    bind('identity.username', value.username);
    bind('identity.role', value.role);
    renderMembershipSelector(value);
  }

  function renderMembershipSelector(me) {
    var wrap = q('#membership-selector-wrap');
    var readonly = q('#membership-readonly');
    var submit = q('#context-submit');
    var select = q('#membership-selector');
    if (!wrap || !readonly || !submit || !select) return;
    removeChildren(select);
    var memberships = me.memberships || [];
    if (memberships.length > 1) {
      memberships.forEach(function (membership) {
        var option = create('option', '', membership.label);
        option.value = membership.id;
        option.selected = membership.id === me.currentMembershipId;
        select.appendChild(option);
      });
      wrap.hidden = false;
      readonly.hidden = true;
      submit.hidden = false;
      bind('scope.membershipHelp', 'Choose the membership/session that should define this signed-in portal view.');
    } else {
      wrap.hidden = true;
      readonly.hidden = false;
      submit.hidden = true;
      readonly.textContent = memberships.length === 1 ? 'One membership/session is active; this context is read-only.' : 'Membership options were not supplied; this context is read-only.';
    }
  }

  function sourceStatus(source) {
    if (!state.live) return 'fallback';
    var text = (String(source.freshness || '') + ' ' + String(source.finality || '')).toLowerCase();
    if (text.indexOf('stale') >= 0) return 'stale';
    if (text.indexOf('partial') >= 0 || text.indexOf('incomplete') >= 0) return 'partial';
    if (state.pageError) return 'error';
    if (source.freshness === 'Unavailable') return 'unavailable';
    return 'live';
  }

  function renderRoute(data) {
    var route = data.route;
    var ambiguous = data.ambiguous;
    var live = state.live;
    var stateLabel;
    var statusLabel;
    var statusDetail;
    var execution;
    var capacity;
    var boundaryHeadline;
    var boundaryDetail;
    var capacityHeadline;
    var evidence;
    if (ambiguous && live) {
      stateLabel = 'Select a model';
      statusLabel = 'Select a model to resolve the route';
      statusDetail = 'More than one route is available. Select a model alias before reading status, execution, or capacity.';
      execution = 'Unavailable — select a model';
      capacity = 'Unavailable — select a model';
      boundaryHeadline = 'Route selection required';
      boundaryDetail = 'More than one route is available. Select a model alias to hydrate execution and capacity from one selected route and service plan.';
      capacityHeadline = 'Select a model to resolve execution and capacity.';
      evidence = { state: 'Select a model', label: 'Select a model to resolve the route', detail: statusDetail, freshness: 'Unknown', source: 'Route evidence unavailable', noteTitle: 'Select a model', noteDetail: 'More than one route is available. Select a model alias before interpreting probe evidence or freshness.' };
    } else if (!route && live) {
      stateLabel = 'Unavailable';
      statusLabel = 'Route evidence is unavailable';
      statusDetail = 'The connected source did not provide a selected route; no callability or capacity claim is made.';
      execution = 'Unavailable — selected route not supplied';
      capacity = 'Unavailable — service plan not supplied';
      boundaryHeadline = 'Selected route unavailable';
      boundaryDetail = 'Execution and capacity are not shown until the backend supplies a selected route and service plan.';
      capacityHeadline = 'Execution and capacity details are unavailable.';
      evidence = routeEvidence(null);
    } else {
      route = route || FALLBACK.route;
      evidence = live ? routeEvidence(route) : { state: humanStatus(route.state), label: route.statusLabel || humanStatus(route.state), detail: route.statusDetail, freshness: 'Unknown', source: 'Target registry + inference ledger · static fallback', noteTitle: route.attentionTitle, noteDetail: route.attentionDetail };
      stateLabel = evidence.state;
      statusLabel = evidence.label;
      statusDetail = evidence.detail;
      execution = live ? (route.executionClass || 'Unavailable — selected route did not supply it') : route.executionClass;
      capacity = live ? (route.capacityMode || 'Unavailable — service plan did not supply it') : route.capacityMode;
      boundaryHeadline = live ? [execution, capacity].filter(function (item) { return item && item.indexOf('Unavailable') !== 0; }).join(' · ') || 'Selected route boundary unavailable' : route.boundaryHeadline;
      boundaryDetail = live ? route.boundaryDetail || 'Execution and service mode are operator-declared for this selected route; live readiness is reported separately.' : route.boundaryDetail;
      capacityHeadline = live && execution && capacity && execution.indexOf('Unavailable') !== 0 && capacity.indexOf('Unavailable') !== 0 ? execution + ' / ' + capacity : (live ? 'Execution and capacity details are unavailable.' : route.capacityHeadline);
    }
    bind('route.stateLabel', stateLabel);
    bind('route.statusLabel', statusLabel);
    bind('route.statusDetail', statusDetail);
    bind('route.attentionTitle', evidence.noteTitle);
    bind('route.attentionDetail', evidence.noteDetail);
    bind('route.freshness', evidence.freshness);
    bind('route.evidenceSource', evidence.source);
    bind('route.evidenceDetail', evidence.noteDetail);
    bind('route.executionClass', execution);
    bind('route.capacityMode', capacity);
    bind('route.boundaryHeadline', boundaryHeadline);
    bind('route.boundaryDetail', boundaryDetail);
    bind('route.capacityHeadline', capacityHeadline);
    bind('route.lastSuccess', route ? formatDate(route.lastSuccessAt) : 'Unknown');
    bind('route.lastObservation', route ? formatDate(route.lastObservationAt) : 'Unknown');
    bind('route.modelAlias', route && route.modelAlias ? route.modelAlias : (live ? 'Approved alias unavailable' : 'configured-alias'));
    bind('route.selectionHelp', ambiguous && live ? 'More than one route is available. Select a model alias before interpreting status or capacity.' : 'Execution and capacity are shown only for the selected route and service plan.');
    all('.route-hero').forEach(function (node) { setState(node, ambiguous ? 'unknown' : String(stateLabel || 'unknown').toLowerCase().replace(/\s+/g, '-')); });
    renderRouteOptions(data);
    renderRouteTable(data);
  }

  function optionList(select, values, firstLabel, selected) {
    if (!select) return;
    removeChildren(select);
    var first = create('option', '', firstLabel);
    first.value = '';
    first.selected = !selected;
    select.appendChild(first);
    values.forEach(function (value) {
      var option = create('option', '', value);
      option.value = value;
      option.selected = String(value) === String(selected || '');
      select.appendChild(option);
    });
  }

  function renderRouteOptions(data) {
    var values = (data.routes || []).map(function (route) { return route.modelAlias; }).filter(Boolean);
    var models = (data.breakdowns.models || []).map(function (item) { return firstValue(item, ['alias', 'model_alias']); }).filter(Boolean);
    values = values.concat(models.filter(function (value) { return values.indexOf(value) < 0; }));
    optionList(q('#route-model-select'), values, 'Select a model alias', state.routeSelection);
    optionList(q('#usage-model'), values, 'All configured aliases', state.routeSelection);
  }

  function renderRouteTable(data) {
    var body = q('#routes-table tbody');
    if (!body) return;
    removeChildren(body);
    if (!state.live || !(data.routes || []).length) {
      body.appendChild(emptyRow(7, state.live ? 'No route registry snapshot supplied.' : 'No route registry snapshot supplied.'));
      return;
    }
    data.routes.forEach(function (route) {
      var row = create('tr');
      var isAmbiguous = data.ambiguous && !state.routeSelection;
      var evidence = routeEvidence(route);
      appendCell(row, route.modelAlias || 'Alias unavailable');
      appendCell(row, isAmbiguous ? 'Select a model' : evidence.label);
      appendCell(row, isAmbiguous ? 'Select a model' : route.executionClass || 'Unavailable');
      appendCell(row, isAmbiguous ? 'Select a model' : route.capacityMode || 'Unavailable');
      appendCell(row, formatDate(route.lastSuccessAt));
      appendCell(row, formatDate(route.lastObservationAt));
      appendCell(row, isAmbiguous ? 'Select a model' : evidence.freshness || 'Unknown');
      body.appendChild(row);
    });
  }

  function appendCell(row, value, className) {
    var cell = create('td', className || '', value);
    row.appendChild(cell);
    return cell;
  }

  function emptyRow(colspan, message) {
    var row = create('tr');
    var cell = create('td', 'table-empty', message);
    cell.colSpan = colspan;
    row.appendChild(cell);
    return row;
  }

  function renderUsage(data) {
    var usage = data.usage || {};
    var logical = usage.logicalRequests;
    bind('usage.requests', formatCount(logical));
    bind('usage.successful', formatCount(usage.successfulRequests));
    bind('usage.outcomes', formatCount(usage.successfulRequests) + ' / ' + formatCount(usage.failedRequests) + ' / ' + formatCount(usage.blockedRequests));
    bind('usage.p95', formatMilliseconds(usage.p95LatencyMs));
    bind('usage.throughput', state.live && usage.throughput !== null ? formatThroughput(usage.throughput) : 'Unavailable — no throughput_rps supplied');
    bind('usage.throughputHelp', state.live && usage.throughput !== null ? 'Reported by the direct usage ledger as throughput_rps, in requests per second for this period.' : 'Unavailable until a connected ledger reports throughput_rps.');
    bind('usage.concurrency', state.live && usage.peakConcurrency !== null ? formatConcurrency(usage.peakConcurrency) : 'Unavailable — no peak_concurrency supplied');
    bind('usage.concurrencyHelp', state.live && usage.peakConcurrency !== null ? 'Reported by the direct usage ledger as peak_concurrency, a count of concurrent logical requests in this period.' : 'Unavailable until a connected ledger reports peak_concurrency.');
    bind('usage.tokensFinality', formatCount(usage.tokens && usage.tokens.total) + ' / ' + formatTokenFinality(usage.tokenFinality));
    bind('usage.tokenDetail', formatCount(usage.tokens && usage.tokens.total) + ' / ' + formatTokenFinality(usage.tokenFinality));
    bind('allocation.shared', data.allocation.shared);
    bind('allocation.dedicated', data.allocation.dedicated);
    bind('allocation.source', data.allocation.source);
    bind('allocation.finality', data.allocation.finality);
    bind('allocation.contextDetail', data.allocation.contextDetail);
    bind('attribution.serviceAccount', data.attribution.serviceAccount);
    var status = sourceStatus(data.source);
    var usageState = q('#usage-state');
    setState(usageState, status);
    bind('usage.stateLabel', logical === 0 ? 'Zero recorded requests' : (status === 'fallback' ? 'Safe preview state' : data.source.label));
    bind('usage.stateDetail', logical === 0 ? 'No inference requests were recorded for this project/environment. Signing into the portal does not create model usage.' : (status === 'fallback' ? 'No live usage snapshot is connected.' : data.source.detail));
    all('[data-empty-message]').forEach(function (node) { node.hidden = logical !== 0; });
    renderExports(data);
    renderAttribution(data);
    renderTrend(data);
    renderBreakdowns(data);
    renderRequests(data);
  }

  function renderExports(data) {
    var meta = data && data.exportMeta || { available: false, formats: [] };
    var formats = Array.isArray(meta.formats) ? meta.formats : [];
    var finality = data && data.source ? humanStatus(data.source.finality) : 'Unknown';
    var status;
    if (!state.live) status = 'Exports require a connected final usage snapshot.';
    else if (!meta.available) status = 'Exports are unavailable while this usage snapshot is ' + finality.toLowerCase() + '.';
    else if (!formats.length) status = 'The backend did not advertise an export format for this snapshot.';
    else status = 'Final scoped snapshot · ' + formats.map(function (format) { return format.toUpperCase(); }).join(' and ') + ' available.';
    bind('export.status', status);
    all('[data-export-format]').forEach(function (button) {
      var format = String(button.getAttribute('data-export-format') || '').toLowerCase();
      var enabled = state.live && meta.available === true && formats.indexOf(format) >= 0;
      button.disabled = !enabled;
      button.title = enabled ? 'Download the final scoped ' + format.toUpperCase() + ' export' : status;
    });
  }

  function renderAttribution(data) {
    var body = q('#service-attribution-body');
    if (!body) return;
    removeChildren(body);
    var rows = data.breakdowns.serviceAccounts || [];
    if (!state.live || !rows.length) {
      body.appendChild(emptyRow(4, 'No service-account attribution supplied by this source.'));
      return;
    }
    rows.forEach(function (item) {
      var row = create('tr');
      appendCell(row, stringValue(firstValue(item, ['name', 'service_account', 'serviceAccount']), 'Unknown'));
      appendCell(row, formatCount(firstValue(item, ['requests', 'logical_requests'])));
      appendCell(row, formatCount(firstValue(item, ['tokens', 'total_tokens'])));
      appendCell(row, stringValue(firstValue(item, ['source', 'source_label']), 'Usage ledger'));
      body.appendChild(row);
    });
  }

  function renderTrend(data) {
    var points = data.trend.points || [];
    bind('trend.unit', data.trend.unit);
    var tableBody = q('#trend-table tbody');
    if (tableBody) {
      removeChildren(tableBody);
      if (!state.live || !points.length) tableBody.appendChild(emptyRow(5, 'No trend points supplied.'));
      else points.forEach(function (point) {
        var row = create('tr');
        appendCell(row, point.label);
        appendCell(row, formatCount(point.requests));
        appendCell(row, formatCount(point.tokens));
        appendCell(row, formatPercent(point.successRate));
        appendCell(row, formatMilliseconds(point.p95LatencyMs));
        tableBody.appendChild(row);
      });
    }
    var svg = q('#usage-chart');
    var empty = q('#chart-empty');
    if (!svg) return;
    var grid = q('.chart-grid', svg);
    var pointGroup = q('.chart-points', svg);
    var polyline = q('.chart-line', svg);
    removeChildren(grid);
    removeChildren(pointGroup);
    polyline.setAttribute('points', '');
    if (!state.live || !points.length) {
      setHidden(empty, false);
      q('#trend-description').textContent = 'No connected trend data is available for this period.';
      return;
    }
    setHidden(empty, true);
    var width = 900;
    var height = 240;
    var left = 42;
    var right = 24;
    var top = 22;
    var bottom = 35;
    var max = Math.max.apply(null, points.map(function (point) { return point.requests === null ? 0 : point.requests; }).concat([1]));
    var coordinates = points.map(function (point, index) {
      var x = points.length === 1 ? (width + left - right) / 2 : left + (index * (width - left - right) / (points.length - 1));
      var value = point.requests === null ? 0 : point.requests;
      var y = height - bottom - ((value / max) * (height - top - bottom));
      return { x: x, y: y, value: value, label: point.label };
    });
    [0, .25, .5, .75, 1].forEach(function (ratio) {
      var y = height - bottom - ratio * (height - top - bottom);
      var line = doc.createElementNS('http://www.w3.org/2000/svg', 'line');
      line.setAttribute('x1', left); line.setAttribute('x2', width - right); line.setAttribute('y1', y); line.setAttribute('y2', y);
      grid.appendChild(line);
    });
    polyline.setAttribute('points', coordinates.map(function (point) { return point.x + ',' + point.y; }).join(' '));
    coordinates.forEach(function (point) {
      var circle = doc.createElementNS('http://www.w3.org/2000/svg', 'circle');
      circle.setAttribute('cx', point.x); circle.setAttribute('cy', point.y); circle.setAttribute('r', 4);
      pointGroup.appendChild(circle);
    });
    q('#trend-description').textContent = 'Requests over time. The accessible table below contains the same ' + points.length + ' period values.';
  }

  function renderBreakdowns(data) {
    var modelsBody = q('#model-breakdown-table tbody');
    var projectsBody = q('#project-breakdown-table tbody');
    if (modelsBody) {
      removeChildren(modelsBody);
      if (!state.live || !data.breakdowns.models.length) modelsBody.appendChild(emptyRow(5, 'No model breakdown supplied.'));
      else data.breakdowns.models.forEach(function (item) {
        var row = create('tr');
        appendCell(row, stringValue(firstValue(item, ['alias', 'model_alias']), 'Unknown'));
        appendCell(row, stringValue(firstValue(item, ['executed_model', 'executedModel']), 'Unknown'));
        appendCell(row, formatCount(firstValue(item, ['requests', 'logical_requests'])));
        appendCell(row, formatCount(firstValue(item, ['tokens', 'total_tokens'])));
        appendCell(row, formatPercent(firstValue(item, ['share'])));
        modelsBody.appendChild(row);
      });
    }
    if (projectsBody) {
      removeChildren(projectsBody);
      if (!state.live || !data.breakdowns.projects.length) projectsBody.appendChild(emptyRow(4, 'No project breakdown supplied.'));
      else data.breakdowns.projects.forEach(function (item) {
        var row = create('tr');
        appendCell(row, stringValue(firstValue(item, ['name', 'project']), 'Unknown'));
        appendCell(row, formatCount(firstValue(item, ['requests', 'logical_requests'])));
        appendCell(row, formatCount(firstValue(item, ['tokens', 'total_tokens'])));
        appendCell(row, formatPercent(firstValue(item, ['share'])));
        projectsBody.appendChild(row);
      });
    }
  }

  function renderRequests(data) {
    var body = q('#recent-requests-table tbody');
    if (!body) return;
    removeChildren(body);
    if (!state.live || !data.requests.length) {
      body.appendChild(emptyRow(9, 'No safe request metadata supplied.'));
      return;
    }
    data.requests.forEach(function (item) {
      var request = item || {};
      var row = create('tr');
      var idCell = create('td');
      var id = stringValue(firstValue(request, ['request_id', 'requestId', 'id']), 'Unavailable');
      var code = create('code', 'request-id', id);
      idCell.appendChild(code);
      if (id !== 'Unavailable') {
        var copy = create('button', 'copy-id', 'Copy');
        copy.type = 'button'; copy.setAttribute('data-copy-value', id); copy.setAttribute('aria-label', 'Copy request ID ' + id);
        idCell.appendChild(copy);
      }
      row.appendChild(idCell);
      appendCell(row, formatDate(firstValue(request, ['started_at', 'startedAt', 'occurred_at', 'occurredAt'])));
      appendCell(row, stringValue(firstValue(request, ['project', 'project_environment']), 'Unknown'));
      appendCell(row, stringValue(firstValue(request, ['service_account', 'serviceAccount', 'service_account_name']), 'Unavailable'));
      appendCell(row, stringValue(firstValue(request, ['model_alias', 'modelAlias']), 'Unknown'));
      appendCell(row, stringValue(firstValue(request, ['executed_model', 'executedModel']), 'Unknown'));
      appendCell(row, stringValue(firstValue(request, ['status']), 'Unknown'));
      appendCell(row, formatMilliseconds(firstValue(request, ['duration_ms', 'durationMs', 'latency_ms', 'latencyMs'])));
      appendCell(row, formatCount(firstValue(request, ['tokens', 'total_tokens'])));
      body.appendChild(row);
    });
  }

  function renderModels(data) {
    var value = data || unavailableCatalogue();
    var status = !state.live ? 'fallback' : value.unavailable ? 'unavailable' : value.models.length ? 'live' : 'empty';
    var stateNode = q('#models-state');
    setState(stateNode, status);
    bind('models.source', value.source);
    bind('models.stateLabel', !state.live ? 'Catalogue preview only' : value.unavailable ? 'Catalogue unavailable' : value.models.length ? 'Connected model catalogue' : 'No catalogue entries');
    bind('models.stateDetail', !state.live ? 'No model rows are fabricated in this preview. Connect the catalogue to see real releases and eligible profiles.' : value.detail);
    var body = q('#models-table tbody');
    if (body) {
      removeChildren(body);
      var search = String(state.modelSearch || '').toLowerCase();
      var mode = String(state.modelModeFilter || '').toLowerCase();
      var models = (value.models || []).filter(function (model) {
        var haystack = [model.name, model.slug, model.release, model.summary].join(' ').toLowerCase();
        var modes = model.profiles.map(function (profile) { return String(profile.mode || '').toLowerCase(); });
        return (!search || haystack.indexOf(search) >= 0) && (!mode || modes.indexOf(mode) >= 0);
      });
      if (!state.live || value.unavailable || !models.length) body.appendChild(emptyRow(7, !state.live ? 'No connected model catalogue supplied.' : value.unavailable ? 'The model catalogue is unavailable; no availability claim is made.' : 'No model releases match these filters.'));
      else models.forEach(function (model) {
        var row = create('tr');
        var identity = create('td'); identity.appendChild(create('span', 'table-primary', model.name)); identity.appendChild(create('span', 'table-secondary', model.slug + ' · ' + model.release)); row.appendChild(identity);
        appendCell(row, model.lifecycle);
        appendCell(row, model.modalities + ' · context ' + model.context);
        appendCell(row, profileModeSummary(model, 'Shared'));
        appendCell(row, profileModeSummary(model, 'Dedicated'));
        appendCell(row, model.source + ' · ' + model.freshness);
        var action = create('td'); var link = create('a', 'table-action', 'Inspect'); link.href = '/app/models/' + encodeURIComponent(model.slug); link.setAttribute('data-view-link', ''); action.appendChild(link); row.appendChild(action);
        body.appendChild(row);
      });
    }
    renderModelDetail(state.modelDetail);
  }

  function profileModeSummary(model, mode) {
    var profiles = (model.profiles || []).filter(function (profile) { return profile.mode === mode; });
    if (!profiles.length) return 'Not supplied';
    return profiles.map(function (profile) { return profile.availability; }).filter(Boolean).join(' · ') || 'Unknown';
  }

  function renderModelDetail(model) {
    var detail = q('#model-detail-view');
    var catalogue = q('#models-catalogue-view');
    var route = state.resource || {};
    var shouldShow = route.kind === 'model-detail';
    if (detail) detail.hidden = !shouldShow;
    if (catalogue) catalogue.hidden = shouldShow;
    if (!shouldShow) return;
    var value = model;
    if (!value) {
      bindText('#model-detail-title', 'Model detail unavailable');
      bindText('#model-detail-lead', state.live ? 'The requested model release was not supplied by the connected catalogue.' : 'Connect the model catalogue to inspect release facts.');
      fillModelDetailUnknown();
      return;
    }
    bindText('#model-detail-title', value.name);
    bindText('#model-detail-lead', value.summary);
    bindText('#model-detail-release', value.release);
    bindText('#model-detail-lifecycle', value.lifecycle);
    bindText('#model-detail-context', value.context);
    bindText('#model-detail-modalities', value.modalities);
    bindText('#model-detail-source', value.source + ' · ' + value.freshness);
    bindText('#model-detail-capabilities', value.capabilitiesText + (value.licence !== 'Unknown' ? ' · Licence: ' + value.licence : '') + (value.support !== 'Unknown' ? ' · Support: ' + value.support : ''));
    bindText('#model-detail-availability', value.profiles.length ? 'The registry supplied ' + value.profiles.length + ' eligible profile record' + (value.profiles.length === 1 ? '' : 's') + '. Availability remains profile-specific.' : 'No eligible profile evidence was supplied for this release.');
    var configure = q('#model-configure-link');
    if (configure) { configure.href = '/app/endpoints/new?model=' + encodeURIComponent(value.slug); configure.setAttribute('data-view-link', ''); configure.disabled = !state.live || !value.profiles.length; }
    var body = q('#model-profiles-table tbody');
    if (!body) return;
    removeChildren(body);
    if (!state.live || !value.profiles.length) { body.appendChild(emptyRow(6, !state.live ? 'No connected profile evidence supplied.' : 'No eligible execution profiles supplied.')); return; }
    value.profiles.forEach(function (profile) {
      var row = create('tr');
      appendCell(row, profile.mode);
      var identity = create('td'); identity.appendChild(create('span', 'table-primary', profile.name)); if (profile.executionClass !== 'Unknown') identity.appendChild(create('span', 'table-secondary', profile.executionClass)); row.appendChild(identity);
      appendCell(row, profile.capacity + (profile.price !== 'Unknown' ? ' · ' + profile.price : ''));
      appendCell(row, profile.availability);
      appendCell(row, profile.commercial + (profile.assumptions ? ' · ' + profile.assumptions : ''));
      var action = create('td'); var link = create('a', 'table-action', profile.eligible ? 'Configure' : 'Not eligible'); link.href = profile.eligible ? '/app/endpoints/new?model=' + encodeURIComponent(value.slug) + '&mode=' + encodeURIComponent(String(profile.mode).toLowerCase()) + '&profile=' + encodeURIComponent(profile.id) : '/app/docs'; if (profile.eligible) link.setAttribute('data-view-link', ''); action.appendChild(link); row.appendChild(action); body.appendChild(row);
    });
  }

  function bindText(selector, value) {
    var node = q(selector);
    if (node) node.textContent = value === undefined || value === null || value === '' ? 'Unknown' : String(value);
  }

  function fillModelDetailUnknown() {
    ['#model-detail-release', '#model-detail-lifecycle', '#model-detail-context', '#model-detail-modalities', '#model-detail-source', '#model-detail-capabilities', '#model-detail-availability'].forEach(function (selector) { bindText(selector, 'Unknown'); });
    var body = q('#model-profiles-table tbody'); if (body) { removeChildren(body); body.appendChild(emptyRow(6, 'No connected profile evidence supplied.')); }
  }

  function commercialLabel(value) {
    var key = String(value || '').toLowerCase().replace(/[\s-]+/g, '_');
    var labels = { paid: 'Paid', paid_shared: 'Paid shared', payment_required: 'Payment required', action_required: 'Payment action required', not_required: 'Payment not required', quote_pending: 'Quote pending', quoted: 'Quote offered', quote_accepted: 'Quote accepted', pending: 'Pending', not_configured: 'Not configured', sandbox: 'Sandbox', past_due: 'Past due', processing: 'Processing' };
    return labels[key] || (value ? humanStatus(value) : 'Unknown');
  }

  function runtimeLabel(endpoint) {
    if (!endpoint) return 'Unknown';
    return endpoint.evidence && endpoint.evidence.label ? endpoint.evidence.label : endpoint.runtimeStatus || 'Unknown';
  }

  function renderEndpoints(data) {
    var value = data || unavailableEndpoints();
    var endpoints = value.endpoints || [];
    var stateNode = q('#endpoints-state');
    setState(stateNode, !state.live ? 'fallback' : value.unavailable ? 'unavailable' : endpoints.length ? 'live' : 'empty');
    bind('endpoints.source', value.source);
    bind('endpoints.stateLabel', !state.live ? 'Endpoint preview only' : value.unavailable ? 'Endpoint inventory unavailable' : endpoints.length ? 'Connected endpoint inventory' : 'No endpoints configured');
    bind('endpoints.stateDetail', !state.live ? 'No endpoint rows are fabricated in this preview. Connect the endpoint inventory to inspect real aliases.' : value.detail);
    var body = q('#endpoints-table tbody');
    if (body) {
      removeChildren(body);
      if (!state.live || value.unavailable || !endpoints.length) body.appendChild(emptyRow(8, !state.live ? 'No connected endpoint records supplied.' : value.unavailable ? 'The endpoint inventory is unavailable; no readiness or commercial claim is made.' : 'No endpoints are configured in this scope.'));
      else endpoints.forEach(function (endpoint) {
        var row = create('tr');
        var identity = create('td'); identity.appendChild(create('span', 'table-primary', endpoint.alias)); identity.appendChild(create('span', 'table-secondary', endpoint.modelName + ' · ' + endpoint.release)); row.appendChild(identity);
        appendCell(row, endpoint.mode);
        appendCell(row, endpoint.environment);
        appendCell(row, runtimeLabel(endpoint));
        appendCell(row, commercialLabel(endpoint.commercialRaw || endpoint.commercial));
        appendCell(row, endpoint.mode === 'Shared' ? endpoint.allowance.shared : endpoint.capacity);
        appendCell(row, formatDate(endpoint.lastObservedAt));
        var action = create('td'); var link = create('a', 'table-action', 'Inspect'); link.href = '/app/endpoints/' + encodeURIComponent(endpoint.id); link.setAttribute('data-view-link', ''); action.appendChild(link); row.appendChild(action);
        body.appendChild(row);
      });
    }
    var emptyNext = q('#endpoint-empty-next'); if (emptyNext) emptyNext.hidden = !!endpoints.length || value.unavailable;
    renderEndpointDetail(state.endpointDetail);
  }

  function renderEndpointDetail(endpoint) {
    var detail = q('#endpoint-detail-view');
    var list = q('#endpoints-list-view');
    var shouldShow = state.resource && state.resource.kind === 'endpoint-detail';
    if (detail) detail.hidden = !shouldShow;
    if (list) list.hidden = shouldShow;
    if (!shouldShow) return;
    if (!endpoint) {
      bindText('#endpoint-detail-title', 'Endpoint detail unavailable'); bindText('#endpoint-detail-lead', 'The requested endpoint was not supplied by the connected inventory.');
      ['#endpoint-detail-state-label', '#endpoint-detail-state-detail', '#endpoint-detail-model', '#endpoint-detail-alias', '#endpoint-detail-mode', '#endpoint-detail-environment', '#endpoint-detail-execution', '#endpoint-detail-path', '#endpoint-detail-source', '#endpoint-detail-lifecycle', '#endpoint-detail-observed', '#endpoint-detail-freshness', '#endpoint-detail-evidence', '#endpoint-detail-commercial', '#endpoint-detail-allowance', '#endpoint-detail-capacity'].forEach(function (selector) { bindText(selector, 'Unknown'); });
      return;
    }
    var evidence = endpoint.evidence || { label: 'Unknown', detail: 'No readiness evidence supplied.', freshness: 'Unknown', source: 'Unknown', state: 'Unknown', noteDetail: 'No readiness evidence supplied.' };
    bindText('#endpoint-detail-title', endpoint.alias);
    bindText('#endpoint-detail-lead', endpoint.modelName + ' · ' + endpoint.mode + ' · ' + endpoint.environment);
    bindText('#endpoint-detail-state-label', runtimeLabel(endpoint));
    bindText('#endpoint-detail-state-detail', evidence.detail);
    bindText('#endpoint-detail-model', endpoint.modelName + ' · ' + endpoint.release);
    bindText('#endpoint-detail-alias', endpoint.alias);
    bindText('#endpoint-detail-mode', endpoint.mode);
    bindText('#endpoint-detail-environment', endpoint.environment);
    bindText('#endpoint-detail-execution', endpoint.executionClass);
    bindText('#endpoint-detail-path', endpoint.endpointPath);
    bindText('#endpoint-detail-source', endpoint.source + ' · ' + endpoint.freshness);
    bindText('#endpoint-detail-lifecycle', endpoint.lifecycle);
    bindText('#endpoint-detail-observed', formatDate(endpoint.lastObservedAt));
    bindText('#endpoint-detail-freshness', evidence.freshness || endpoint.freshness);
    bindText('#endpoint-detail-evidence', evidence.source + ' · ' + (endpoint.route && endpoint.route.probeStatus ? humanStatus(endpoint.route.probeStatus) : 'No probe status supplied'));
    bindText('#endpoint-detail-commercial', commercialLabel(endpoint.commercialRaw || endpoint.commercial));
    bindText('#endpoint-detail-allowance', endpoint.allowance.shared);
    bindText('#endpoint-detail-capacity', endpoint.capacity);
    bindText('#endpoint-detail-readiness-note', evidence.noteDetail || 'Readiness is based on selected endpoint evidence, not payment alone.');
    var usageLink = q('#endpoint-usage-link'); if (usageLink) { usageLink.href = endpoint.usageHref || '/app/usage'; usageLink.setAttribute('data-view-link', ''); }
    var code = q('#endpoint-api-example');
    var gateway = endpoint.gatewayBaseUrl || (state.me && state.me.gatewayBaseUrl);
    if (code) {
      if (!gateway || !endpoint.alias || endpoint.alias === 'Alias unavailable') code.textContent = '# API example unavailable until the backend supplies a safe gateway URL and model alias.';
      else code.textContent = 'curl "' + gateway + endpoint.endpointPath + '" \\\n  -H "Authorization: Bearer $ALZETTE_API_KEY" \\\n  -H "Content-Type: application/json" \\\n  -d \'{"model":' + JSON.stringify(endpoint.alias) + ',"messages":[{"role":"user","content":"Say hello in one sentence."}]}\'';
    }
    var actions = q('#endpoint-capacity-actions');
    if (actions) {
      removeChildren(actions);
      var runtimeRaw = String(firstValue(endpoint.raw, ['runtime.state', 'runtime_status', 'runtime_state']) || '').toLowerCase();
      if (endpoint.mode === 'Dedicated' && ['ready', 'degraded'].indexOf(runtimeRaw) >= 0) {
        var capacityButton = create('button', 'button button--quiet', 'Request more capacity');
        capacityButton.type = 'button'; capacityButton.setAttribute('data-capacity-action', endpoint.id); actions.appendChild(capacityButton);
      } else if (endpoint.mode === 'Dedicated') {
        actions.appendChild(create('span', 'form-help', 'Capacity changes become available after this dedicated endpoint reaches a validated ready or degraded state.'));
      } else {
        var link = create('a', 'button button--quiet', 'Review available models');
        link.href = '/app/models'; link.setAttribute('data-view-link', ''); actions.appendChild(link);
      }
    }
    setState(q('#endpoint-detail-state'), String(evidence.state || 'unknown').toLowerCase().replace(/\s+/g, '-'));
  }

  function renderBilling(data) {
    var value = data || unavailableBilling();
    var stateKey = !state.live ? 'not_configured' : value.unavailable ? 'unavailable' : value.state || 'not_configured';
    var stateNode = q('#billing-state');
    setState(stateNode, stateKey);
    bind('billing.source', value.source);
    var labels = { not_configured: 'Billing not configured', sandbox: 'Billing sandbox', loading: 'Loading billing', empty: 'No billing records', success_return: 'Payment confirmation pending', cancellation: 'Checkout cancelled', stale: 'Billing needs refresh', permission: 'Billing permission required', error: 'Billing unavailable' };
    bind('billing.stateLabel', labels[stateKey] || commercialLabel(stateKey));
    bind('billing.stateDetail', value.detail);
    bindText('#billing-account', value.account);
    bindText('#billing-commercial-state', commercialLabel(value.commercial));
    bindText('#billing-tax-status', value.taxStatus);
    bindText('#billing-checkout-state', value.checkout);
    bindText('#billing-confirmed-at', formatDate(value.confirmedAt));
    bindText('#billing-action-note', value.canManage ? 'Manage billing opens Alzette’s hosted billing portal in a new server-authorized session.' : value.unavailable ? 'Billing data is unavailable; no payment or invoice claim is made.' : 'Your role cannot manage billing, or the backend has not enabled that action.');
    var manage = q('#billing-manage'); if (manage) { manage.disabled = !state.live || value.unavailable || value.canManage !== true; manage.title = manage.disabled ? 'Billing management is unavailable for this session.' : ''; }
    var retry = q('#billing-retry'); if (retry) { retry.hidden = !state.live || ['unavailable', 'stale', 'error', 'permission'].indexOf(stateKey) < 0; }
    var source = q('#billing-invoices-source'); if (source) source.textContent = value.invoices.length ? 'Connected billing records' : 'No invoice source supplied';
    var body = q('#billing-invoices-table tbody');
    if (!body) return;
    removeChildren(body);
    if (!state.live || value.unavailable || !value.invoices.length) { body.appendChild(emptyRow(6, !state.live ? 'No connected billing records supplied.' : value.unavailable ? 'Billing records are unavailable; no invoice claim is made.' : 'No invoice records supplied.')); return; }
    value.invoices.forEach(function (invoice) {
      var item = invoice || {}; var row = create('tr');
      appendCell(row, stringValue(firstValue(item, ['reference', 'number', 'display_reference']), 'Unavailable'));
      appendCell(row, formatFact(firstValue(item, ['period', 'billing_period']) || (firstValue(item, ['due_at']) ? 'Due ' + formatDate(firstValue(item, ['due_at'])) : null), 'Unknown'));
      appendCell(row, commercialLabel(firstValue(item, ['status', 'state'])));
      var amountMinor = String(firstValue(item, ['status', 'state']) || '').toLowerCase() === 'paid' ? firstValue(item, ['amount_paid_minor', 'amount_due_minor']) : firstValue(item, ['amount_due_minor', 'amount_paid_minor']);
      appendCell(row, amountMinor !== null ? (formatMoneyMinor(amountMinor, firstValue(item, ['currency'])) || 'Unknown') : formatFact(firstValue(item, ['amount', 'total']), 'Unknown') + (firstValue(item, ['currency']) ? ' ' + String(firstValue(item, ['currency'])) : ''));
      appendCell(row, formatDate(firstValue(item, ['issued_at', 'created_at'])));
      var cell = create('td'); var url = normalizeHostedUrl(firstValue(item, ['document_url', 'hosted_url', 'url'])); if (url) { var link = create('a', 'table-action', 'Open document'); link.href = url; link.target = '_blank'; link.rel = 'noreferrer'; cell.appendChild(link); } else cell.textContent = 'Unavailable'; row.appendChild(cell); body.appendChild(row);
    });
  }

  function normalizeHostedUrl(value) {
    if (!value) return '';
    try {
      var url = new URL(String(value), window.location.origin);
      if (url.origin === window.location.origin) return url.href;
      var host = url.hostname.toLowerCase();
      if (url.protocol !== 'https:' || (host !== 'stripe.com' && !host.endsWith('.stripe.com'))) return '';
      return url.href;
    } catch (error) { return ''; }
  }

  function configModel() {
    var models = state.catalogue && state.catalogue.models || [];
    return models.filter(function (model) { return model.slug === state.configurator.modelSlug; })[0] || null;
  }

  function configProfile() {
    var model = configModel();
    if (model && !state.configurator.draftId && (!state.configurator.values.alias || state.configurator.values.model_slug !== model.slug)) {
      state.configurator.values.alias = model.endpointAlias;
      state.configurator.values.model_slug = model.slug;
    }
    if (!model) return null;
    return (model.profiles || []).filter(function (profile) { return profile.id === state.configurator.profileId; })[0] || null;
  }

  function setConfigError(message) {
    var error = q('#endpoint-config-error');
    if (!error) return;
    error.textContent = message || '';
    error.hidden = !message;
  }

  function configValue(name, fallback) {
    var node = q('#' + name);
    if (!node) return fallback || '';
    return node.value || fallback || '';
  }

  function renderConfigurator() {
    var view = q('#endpoint-config-view');
    var list = q('#endpoints-list-view');
    var detail = q('#endpoint-detail-view');
    var requestView = q('#endpoint-request-view');
    var active = state.resource && state.resource.kind === 'config';
    if (view) view.hidden = !active;
    if (active) { if (list) list.hidden = true; if (detail) detail.hidden = true; if (requestView) requestView.hidden = true; }
    if (!active) return;
    var locked = !!state.configurator.draftId;
    var models = state.catalogue && state.catalogue.models || [];
    var modelSelect = q('#config-model');
    if (modelSelect) {
      var selectedModel = state.configurator.modelSlug;
      removeChildren(modelSelect);
      var placeholder = create('option', '', state.live ? (models.length ? 'Select a connected model' : 'No connected models supplied') : 'Connect catalogue to choose a model'); placeholder.value = ''; modelSelect.appendChild(placeholder);
      models.forEach(function (model) { var option = create('option', '', model.name + ' · ' + model.release); option.value = model.slug; option.selected = model.slug === selectedModel; modelSelect.appendChild(option); });
      modelSelect.disabled = locked || !state.live || !models.length;
    }
    var model = configModel();
    bindText('#config-scope', state.me ? state.me.projectEnvironment : 'Project / environment');
    var modelFacts = q('#config-model-facts');
    if (modelFacts) modelFacts.textContent = model ? model.summary + ' · Context ' + model.context + ' · ' + model.modalities : (state.live ? 'Choose a connected model to inspect release facts.' : 'No connected model catalogue is available in this preview.');
    all('input[name="deployment_mode"]').forEach(function (input) { input.checked = String(input.value) === String(state.configurator.mode); input.disabled = locked || !model; });
    var modeHelp = q('#config-mode-help');
    if (modeHelp) modeHelp.textContent = model ? (state.configurator.mode ? 'Profiles for ' + humanMode(state.configurator.mode) + ' will be shown in the next step.' : 'Choose Shared or Dedicated to filter eligible profiles.') : 'Choose a model before selecting a mode.';
    var profileSelect = q('#config-profile');
    var profiles = model ? (model.profiles || []).filter(function (profile) { return !state.configurator.mode || profile.mode.toLowerCase() === humanMode(state.configurator.mode).toLowerCase(); }) : [];
    if (profileSelect) {
      removeChildren(profileSelect);
      var profilePlaceholder = create('option', '', profiles.length ? 'Select an eligible profile' : 'No eligible profile supplied'); profilePlaceholder.value = ''; profileSelect.appendChild(profilePlaceholder);
      profiles.forEach(function (profile) { var option = create('option', '', profile.name + ' · ' + profile.availability); option.value = profile.id; option.selected = profile.id === state.configurator.profileId; profileSelect.appendChild(option); });
      profileSelect.disabled = locked || !profiles.length;
    }
    var profile = configProfile();
    var profileFacts = q('#config-profile-facts');
    if (profileFacts) profileFacts.textContent = profile ? [profile.capacity, profile.price !== 'Unknown' ? profile.price : null, profile.assumptions || null].filter(Boolean).join(' · ') || 'No capacity or commercial facts supplied.' : 'Profile capacity, cost, and availability assumptions appear here when supplied by the backend.';
    var values = state.configurator.values || {};
    var configuratorFieldKeys = {
      'config-alias': 'alias',
      'config-use-case': 'use_case',
      'config-context': 'expected_context',
      'config-concurrency': 'expected_concurrency',
      'config-latency': 'latency_intent',
      'config-units': 'capacity_units'
    };
    Object.keys(configuratorFieldKeys).forEach(function (id) {
      var node = q('#' + id);
      var key = configuratorFieldKeys[id];
      if (node && document.activeElement !== node && values[key] !== undefined) node.value = values[key];
    });
    var aliasInput = q('#config-alias'); if (aliasInput) aliasInput.readOnly = true;
    var unitsInput = q('#config-units');
    if (unitsInput) {
      unitsInput.min = profile && profile.minimumCapacityUnits !== null ? String(profile.minimumCapacityUnits) : '1';
      if (profile && profile.maximumCapacityUnits !== null) unitsInput.max = String(profile.maximumCapacityUnits); else unitsInput.removeAttribute('max');
      if (!values.capacity_units && profile && profile.minimumCapacityUnits !== null && document.activeElement !== unitsInput) { values.capacity_units = profile.minimumCapacityUnits; unitsInput.value = String(profile.minimumCapacityUnits); }
    }
    var review = q('#config-review');
    if (review) {
      removeChildren(review);
      var entries = [['Model', model ? model.name + ' · ' + model.release : 'Unknown'], ['Mode', state.configurator.mode ? humanMode(state.configurator.mode) : 'Unknown'], ['Alias', values.alias || 'Unknown'], ['Scope', state.me ? state.me.projectEnvironment : 'Unknown'], ['Profile', profile ? profile.name : 'Unknown'], ['Capacity / estimate', profile ? [profile.capacity, profile.price !== 'Unknown' ? profile.price : null].filter(Boolean).join(' · ') || 'Unknown' : 'Unknown']];
      entries.forEach(function (entry) { var row = create('div'); row.appendChild(create('dt', '', entry[0])); row.appendChild(create('dd', '', entry[1])); review.appendChild(row); });
    }
    var step = Math.max(1, Math.min(6, Number(state.configurator.step) || 1));
    all('[data-config-step-panel]').forEach(function (panel) { panel.hidden = Number(panel.getAttribute('data-config-step-panel')) !== step; });
    all('[data-config-step-indicator]').forEach(function (indicator) { var number = Number(indicator.getAttribute('data-config-step-indicator')); indicator.classList.toggle('is-current', number === step); indicator.classList.toggle('is-complete', number < step); });
    var back = q('#config-back'); if (back) back.disabled = step === 1;
    var next = q('#config-next'); if (next) next.hidden = step === 6;
    var submit = q('#config-submit'); if (submit) { submit.hidden = step !== 6; submit.textContent = state.configurator.mode === 'dedicated' ? 'Request quote / provisioning' : 'Request shared evaluation'; }
    var save = q('#config-save'); if (save) save.disabled = !state.live;
    var draftReady = !!state.configurator.draftId && state.configurator.draftLoaded;
    setState(q('#endpoint-config-state'), draftReady ? 'partial' : (state.configurator.draftId ? 'loading' : (state.live ? 'unknown' : 'fallback')));
    bindText('#endpoint-config-state-label', draftReady ? 'Draft saved' : (state.configurator.draftId ? 'Loading saved draft' : (state.live ? 'Configuration draft' : 'Preview only')));
    bindText('#endpoint-config-state-detail', draftReady ? 'The server accepted and restored this draft. No endpoint, payment, or readiness claim has been made.' : (state.configurator.draftId ? 'Refreshing the server-backed draft before it can be reviewed or submitted.' : (state.live ? 'Complete the guided fields before submitting to the backend.' : 'Connect the catalogue and endpoint configuration API to create a real draft.')));
  }

  function captureConfiguratorValues() {
    var values = state.configurator.values || {};
    values.model_slug = configValue('config-model', state.configurator.modelSlug);
    values.alias = configValue('config-alias');
    values.use_case = configValue('config-use-case');
    values.expected_context = configValue('config-context');
    values.expected_concurrency = configValue('config-concurrency');
    values.latency_intent = configValue('config-latency');
    values.capacity_units = configValue('config-units');
    state.configurator.values = values;
    state.configurator.modelSlug = values.model_slug;
  }

  function validateConfigStep(step) {
    captureConfiguratorValues();
    if (step === 1 && (!state.configurator.modelSlug || !configModel())) return 'Choose a model supplied by the connected catalogue.';
    if (step === 2 && !state.configurator.mode) return 'Choose Shared or Dedicated.';
    if (step === 3 && !state.configurator.values.alias) return 'Enter a stable endpoint alias.';
    if (step === 3 && !/^[A-Za-z0-9][A-Za-z0-9._:-]{0,79}$/.test(state.configurator.values.alias)) return 'Use letters, numbers, dots, underscores, colons, or hyphens; begin with a letter or number.';
    if (step === 4 && state.configurator.values.expected_context && (Number(state.configurator.values.expected_context) < 1 || Number(state.configurator.values.expected_context) > 10000000)) return 'Expected context tokens must be between 1 and 10,000,000.';
    if (step === 4 && state.configurator.values.expected_concurrency && (Number(state.configurator.values.expected_concurrency) < 1 || Number(state.configurator.values.expected_concurrency) > 10000)) return 'Expected concurrency must be between 1 and 10,000.';
    if (step === 5 && (!state.configurator.profileId || !configProfile())) return 'Choose an eligible offer and profile supplied by the connected catalogue.';
    if (step === 5) { var profile = configProfile(); var units = Number(state.configurator.values.capacity_units || (profile && profile.minimumCapacityUnits) || 1); if (profile && ((profile.minimumCapacityUnits !== null && units < profile.minimumCapacityUnits) || (profile.maximumCapacityUnits !== null && units > profile.maximumCapacityUnits))) return 'Capacity units must stay within the selected profile range.'; }
    if (step === 6 && state.configurator.mode === 'dedicated' && !(q('#config-confirm') && q('#config-confirm').checked)) return 'Confirm that dedicated capacity is a request, quote, provisioning, and validation path.';
    return '';
  }

  function normalizeConfiguration(payload) {
    var root = unwrap(payload);
    var value = firstValue(root, ['configuration']) || root;
    var workload = firstValue(value, ['workload']) || {};
    return {
      id: stringValue(firstValue(value, ['id', 'configuration_id']), ''),
      modelSlug: stringValue(firstValue(value, ['model_slug']), ''),
      offerCode: stringValue(firstValue(value, ['offer_code']), ''),
      profileCode: stringValue(firstValue(value, ['profile_code']), ''),
      alias: stringValue(firstValue(value, ['endpoint_alias', 'alias']), ''),
      capacityUnits: numberValue(firstValue(value, ['capacity_units'])),
      workload: workload,
      status: stringValue(firstValue(value, ['status']), 'draft')
    };
  }

  function restoreConfiguration(configuration) {
    if (!configuration || !configuration.id) return false;
    state.configurator.draftId = configuration.id;
    state.configurator.draftLoaded = true;
    state.configurator.modelSlug = configuration.modelSlug;
    var model = configModel();
    var profile = model && (model.profiles || []).filter(function (item) {
      return item.offerCode === configuration.offerCode || (item.profileCode === configuration.profileCode && (!configuration.offerCode || item.offerCode === configuration.offerCode));
    })[0];
    state.configurator.profileId = profile ? profile.id : configuration.offerCode;
    state.configurator.mode = profile ? String(profile.mode).toLowerCase() : '';
    state.configurator.values = {
      model_slug: configuration.modelSlug,
      alias: configuration.alias,
      use_case: stringValue(firstValue(configuration.workload, ['use_case']), ''),
      expected_context: firstValue(configuration.workload, ['expected_context_tokens']),
      expected_concurrency: firstValue(configuration.workload, ['expected_concurrency']),
      latency_intent: stringValue(firstValue(configuration.workload, ['latency_priority']), ''),
      capacity_units: configuration.capacityUnits
    };
    state.configurator.step = 6;
    persistConfiguratorURL(true);
    return true;
  }

  function persistConfiguratorURL(replace) {
    if (!state.resource || state.resource.kind !== 'config') return;
    state.resource.draftId = state.configurator.draftId;
    state.resource.operationId = state.configurator.operationId;
    var params = {
      model: state.configurator.modelSlug,
      mode: state.configurator.mode,
      profile: state.configurator.profileId,
      draft: state.configurator.draftId,
      operation: state.configurator.draftId ? '' : state.configurator.operationId
    };
    var path = queryPath('/app/endpoints/new', params);
    window.history[replace ? 'replaceState' : 'pushState']({ path: path }, '', path);
  }

  function ensureConfigurationOperation() {
    if (!state.configurator.operationId) state.configurator.operationId = window.crypto && typeof window.crypto.randomUUID === 'function' ? window.crypto.randomUUID() : String(Date.now()) + '-' + Math.random().toString(16).slice(2);
    persistConfiguratorURL(true);
    return 'configuration-create-' + state.configurator.operationId;
  }

  function configurationBody(updateOnly) {
    captureConfiguratorValues();
    var values = state.configurator.values || {};
    var profile = configProfile();
    var workload = {};
    if (values.use_case) workload.use_case = String(values.use_case);
    if (values.expected_context) workload.expected_context_tokens = Number(values.expected_context);
    if (values.expected_concurrency) workload.expected_concurrency = Number(values.expected_concurrency);
    if (values.latency_intent) workload.latency_priority = String(values.latency_intent);
    var capacityUnits = Number(values.capacity_units || (profile && profile.minimumCapacityUnits) || 1);
    if (updateOnly) return { capacity_units: capacityUnits, workload: workload };
    return {
      model_slug: state.configurator.modelSlug,
      offer_code: profile ? profile.offerCode : '',
      profile_code: profile ? profile.profileCode : '',
      endpoint_alias: values.alias,
      capacity_units: capacityUnits,
      workload: workload
    };
  }

  async function saveConfiguration(submit) {
    var error = validateConfigStep(submit ? 6 : state.configurator.step);
    if (error) { setConfigError(error); return; }
    setConfigError('');
    var button = submit ? q('#config-submit') : q('#config-save');
    if (button) { button.disabled = true; button.textContent = submit ? 'Submitting…' : 'Saving…'; }
    var body = configurationBody(!!state.configurator.draftId);
    var result;
    var saveOperation = state.configurator.draftId ? 'configuration-update-' + state.configurator.draftId : '';
    if (state.configurator.draftId) result = await request('/api/portal/endpoint-configurations/' + encodeURIComponent(state.configurator.draftId), { method: 'PATCH', body: body, idempotency: saveOperation });
    else result = await request('/api/portal/endpoint-configurations', { method: 'POST', body: body, idempotencyKey: ensureConfigurationOperation() });
    if (button) { button.disabled = false; button.textContent = submit ? 'Submit configuration' : 'Save draft'; }
    if (!result.ok) { if (definitiveResult(result) && saveOperation) clearOperationKey(saveOperation); setConfigError(result.status === 403 ? 'You do not have permission to configure an endpoint in this membership/session.' : responseMessage(result, 'The configuration could not be saved. No endpoint or commercial state was changed.')); return; }
    if (saveOperation) clearOperationKey(saveOperation);
    var response = unwrap(result.data);
    state.configurator.draftId = stringValue(firstValue(response, ['id', 'configuration_id', 'endpoint_configuration_id', 'configuration.id']), state.configurator.draftId);
    state.configurator.draftLoaded = !!state.configurator.draftId;
    state.configurator.operationId = '';
    persistConfiguratorURL(true);
    if (!submit) { renderConfigurator(); showToast('Configuration draft saved by the server.'); return; }
    if (!state.configurator.draftId) { renderConfigurator(); showToast('The server accepted the configuration, but returned no draft reference. No readiness claim was made.'); return; }
    var submitOperation = 'configuration-submit-' + state.configurator.draftId;
    var submitted = await request('/api/portal/endpoint-configurations/' + encodeURIComponent(state.configurator.draftId) + '/submit', { method: 'POST', body: {}, idempotency: submitOperation });
    if (!submitted.ok) { if (definitiveResult(submitted)) clearOperationKey(submitOperation); setConfigError(submitted.status === 403 ? 'You do not have permission to submit this configuration.' : responseMessage(submitted, 'The draft was saved, but submission did not complete. Review the draft and retry.')); renderConfigurator(); return; }
    clearOperationKey(submitOperation);
    var submittedRoot = unwrap(submitted.data);
    var requestId = stringValue(firstValue(submittedRoot, ['deployment_request_id', 'request_id', 'deployment_request.id', 'endpoint.deployment_request_id', 'id']), '');
    if (requestId) { await loadRequestProgress(requestId, true); navigatePath('/app/endpoints/requests/' + encodeURIComponent(requestId), true); }
    else { state.configurator.step = 6; renderConfigurator(); showToast('Configuration submitted. The backend did not return a deployment request reference; readiness remains unknown.'); }
  }

  function progressLabel(value) {
    var key = String(value || 'unknown').toLowerCase().replace(/[\s-]+/g, '_');
    var labels = { draft: 'Draft', submitted: 'Submitted', needs_info: 'Needs information', rejected: 'Rejected', quote_pending: 'Quote pending', offered: 'Quote offered', quote_offered: 'Quote offered', accepted: 'Accepted', expired: 'Expired', superseded: 'Superseded', not_required: 'Payment not required', action_required: 'Payment action required', processing: 'Payment processing', paid: 'Paid', past_due: 'Past due', awaiting_approval: 'Awaiting approval', allocating: 'Allocating', deploying: 'Deploying', validating: 'Validating', ready: 'Validated route ready', degraded: 'Degraded', failed: 'Failed', unavailable: 'Unavailable', unknown: 'Unknown' };
    return labels[key] || humanStatus(value);
  }

  function progressClass(value) {
    var key = String(value || '').toLowerCase();
    if (key === 'ready' || key === 'paid' || key === 'accepted' || key === 'submitted' || key === 'validating') return 'complete';
    if (key === 'failed' || key === 'rejected' || key === 'expired' || key === 'past_due') return 'error';
    if (key === 'needs_info' || key === 'action_required' || key === 'quote_pending' || key === 'offered' || key === 'allocating' || key === 'deploying') return 'attention';
    return 'unknown';
  }

  function renderRequestProgress(progress) {
    var view = q('#endpoint-request-view');
    var list = q('#endpoints-list-view');
    var detail = q('#endpoint-detail-view');
    var config = q('#endpoint-config-view');
    var active = state.resource && state.resource.kind === 'request';
    if (view) view.hidden = !active;
    if (active) { if (list) list.hidden = true; if (detail) detail.hidden = true; if (config) config.hidden = true; }
    if (!active) return;
    var value = progress;
    if (!value) { bindText('#endpoint-request-title', 'Request status unavailable'); bindText('#endpoint-request-lead', 'The requested deployment status was not supplied by the connected service.'); setState(q('#endpoint-request-state'), 'unavailable'); return; }
    bindText('#endpoint-request-title', value.alias !== 'Alias unavailable' ? value.alias : 'Request ' + value.id);
    bindText('#endpoint-request-lead', value.model + ' · configuration, commercial, payment, and infrastructure status are independent.');
    bindText('#endpoint-request-state-label', progressLabel(value.infrastructure.state));
    bindText('#endpoint-request-state-detail', value.infrastructure.detail);
    bindText('#request-next-action', value.nextAction);
    bindText('#request-reference-id', value.id);
    var requestedUnits = value.requestedCapacityUnits;
    var currentUnits = value.currentCapacityUnits;
    var capacityIntent = requestedUnits === null ? 'Unknown' : (currentUnits === null ? requestedUnits + ' capacity unit' + (requestedUnits === 1 ? '' : 's') + ' for a new endpoint' : currentUnits + ' → ' + requestedUnits + ' capacity units');
    bindText('#request-intent-capacity', capacityIntent);
    bindText('#request-intent-use-case', stringValue(firstValue(value.workload, ['use_case']), 'Not specified'));
    var expectedContext = numberValue(firstValue(value.workload, ['expected_context_tokens']));
    bindText('#request-intent-context', expectedContext === null ? 'Not specified' : formatCount(expectedContext) + ' tokens');
    var expectedConcurrency = numberValue(firstValue(value.workload, ['expected_concurrency']));
    bindText('#request-intent-concurrency', expectedConcurrency === null ? 'Not specified' : formatCount(expectedConcurrency) + ' concurrent request' + (expectedConcurrency === 1 ? '' : 's'));
    bindText('#request-intent-priority', humanStatus(firstValue(value.workload, ['latency_priority']) || 'Not specified'));
    setState(q('#endpoint-request-state'), progressClass(value.infrastructure.state));
    ['configuration', 'commercial', 'payment', 'infrastructure'].forEach(function (railName) {
      var rail = value[railName] || normalizeProgressRail(null, 'No status supplied.');
      var node = q('[data-progress-rail="' + railName + '"]');
      if (node) { setState(node, progressClass(rail.state)); var label = q('[data-progress-label="' + railName + '"]', node); var detailNode = q('[data-progress-detail="' + railName + '"]', node); if (label) label.textContent = progressLabel(rail.state); if (detailNode) detailNode.textContent = rail.detail; }
    });
    var actions = q('#request-actions');
    if (!actions) return;
    removeChildren(actions);
    if (value.quoteId && ['offered', 'quote_offered'].indexOf(value.commercial.state) >= 0) { var accept = create('button', 'button button--dark', 'Accept quote'); accept.type = 'button'; accept.setAttribute('data-request-action', 'accept-quote'); actions.appendChild(accept); }
    if (value.paymentRequirementId && ['action_required', 'not_started'].indexOf(value.payment.state) >= 0) { var checkout = create('button', 'button button--dark', 'Pay securely in Stripe'); checkout.type = 'button'; checkout.setAttribute('data-request-action', 'checkout'); actions.appendChild(checkout); }
    if (value.endpointId) { var endpointLink = create('a', 'button button--quiet', 'Open endpoint'); endpointLink.href = '/app/endpoints/' + encodeURIComponent(value.endpointId); endpointLink.setAttribute('data-view-link', ''); actions.appendChild(endpointLink); }
    if (!actions.childNodes.length) { var support = create('a', 'button button--quiet', 'Open docs and support guidance'); support.href = '/app/docs'; support.setAttribute('data-view-link', ''); actions.appendChild(support); }
  }

  function renderDocs(data) {
    var gateway = normalizeGateway((state.me && state.me.gatewayBaseUrl) || data.gatewayBaseUrl);
    var endpoint = gateway ? gateway + '/v1/chat/completions' : 'Endpoint unavailable — safe gateway URL not supplied';
    var alias = data.route && data.route.modelAlias ? data.route.modelAlias : 'approved-model-alias-unavailable';
    bind('docs.endpoint', endpoint);
    var code = q('#first-call-curl-code');
    if (!code) return;
    if (!gateway) {
      code.textContent = '# Endpoint unavailable — safe gateway URL not supplied\n' +
        '# The backend must provide gateway_base_url or public_gateway_url in portal context.\n' +
        '# No control origin or provider URL is inferred.\n' +
        '# Store the issued key as $ALZETTE_API_KEY once the endpoint is available.';
      return;
    }
    var safeEndpoint = endpoint.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\$/g, '\\$');
    var safeAlias = JSON.stringify(alias).replace(/\$/g, '\\$');
    code.textContent = 'curl "' + safeEndpoint + '" \\\n' +
      '  -H "Authorization: Bearer $ALZETTE_API_KEY" \\\n' +
      '  -H "Content-Type: application/json" \\\n' +
      '  -d \'{"model":' + safeAlias + ',"messages":[{"role":"user","content":"Say hello in one sentence."}]}\'';
  }

  function renderData(data) {
    var value = data || blankDashboard('No connected dashboard snapshot was supplied.');
    var source = value.source || blankDashboard().source;
    var scope = value.scope || {};
    bind('account.name', value.account && value.account.name);
    bind('scope.organization', scope.organization || (state.me && state.me.organization));
    bind('scope.project', scope.project || (state.me && state.me.project));
    bind('scope.environment', scope.environment || (state.me && state.me.environment));
    bind('scope.projectEnvironment', [scope.project, scope.environment].filter(Boolean).join(' / ') || (state.me && state.me.projectEnvironment));
    initializePeriodFilters(value.period);
    bind('period.label', value.period.label);
    bind('period.timezone', value.period.timezone);
    renderGlobal(source, sourceStatus(source));
    renderRoute(value);
    renderUsage(value);
    renderDocs(value);
  }

  function canAdmin(access) {
    if (!access) return false;
    var permission = access.permissions || {};
    var role = String(access.role || '').toLowerCase();
    return access.canManage === true || permission.admin === true || permission.can_manage_access === true || permission.manage_access === true || permission.access_admin === true || role === 'admin' || role === 'owner';
  }

  function renderScopeOptions() {
    var container = q('#key-scope-options');
    if (!container || !state.live) return;
    var allowed = state.access && state.access.allowedScopes || [];
    removeChildren(container);
    if (!allowed.length) {
      var unavailable = create('span', 'form-help', 'Allowed scopes unavailable from backend; key issuance is disabled.');
      container.appendChild(unavailable);
      bind('access.scopeHelp', 'The backend did not provide an allowed-scope list for this membership/session.');
      return;
    }
    allowed.forEach(function (scope, index) {
      var label = create('label');
      var input = doc.createElement('input');
      input.type = 'checkbox'; input.name = 'key_scopes'; input.value = scope; input.checked = scope === 'inference:write';
      label.appendChild(input); label.appendChild(doc.createTextNode(' ' + scope)); container.appendChild(label);
    });
    bind('access.scopeHelp', 'Choose only the backend-allowed scopes this application needs. inference:write authorizes the first inference call.');
  }

  function renderExpiryPolicy() {
    var select = q('#key-expiry');
    if (!select || !state.live) return;
    var policy = state.access && state.access.keyPolicy || {};
    var values = arrayValue(firstValue(policy, ['allowed_expiry_days', 'allowed_days', 'expiry_days'])).map(Number).filter(function (days) { return days >= 1 && days <= 365; });
    if (!values.length) values = [30, 90, 365];
    var fallback = numberValue(firstValue(policy, ['default_expiry_days', 'default_days'])) || 90;
    if (values.indexOf(fallback) < 0) values.push(fallback);
    values.sort(function (a, b) { return a - b; });
    removeChildren(select);
    values.forEach(function (days) {
      var option = create('option', '', days === 365 ? '365 days' : days + ' days');
      option.value = days + 'd'; option.selected = days === fallback; select.appendChild(option);
    });
    bind('access.expiryHelp', 'Backend policy: ' + fallback + ' days by default; expiry must be between 1 hour and 365 days.');
  }

  function renderAccess(access) {
    var value = access || unavailableAccess();
    bind('access.source', value.source);
    var unavailable = value.unavailable === true;
    var admin = canAdmin(value) && state.live && !unavailable;
    var permission = q('#access-permission');
    setState(permission, unavailable ? 'unknown' : (admin ? 'allowed' : (state.live ? 'denied' : 'unknown')));
    if (permission) permission.querySelector('span').textContent = unavailable ? value.detail : (admin ? 'Administrator actions are available for this membership/session.' : (state.live ? 'You can view this area, but your role cannot perform this action. Ask an administrator or owner for access.' : 'Connect an authenticated membership/session to determine administrator actions.'));
    var createAccount = q('#service-account-open');
    if (createAccount) { createAccount.disabled = !admin; createAccount.title = admin ? '' : (unavailable ? 'Access metadata unavailable' : 'Administrator permission required'); }
    renderScopeOptions();
    renderExpiryPolicy();
    renderAccounts(value, admin);
    renderKeys(value, admin);
  }

  function accountKeys(access, account) {
    var keys = access.keys.filter(function (key) { return !account.id || !key.serviceAccountId || key.serviceAccountId === account.id; });
    return keys.concat(account.keys || []).filter(function (key, index, list) { return list.findIndex(function (item) { return item.prefix === key.prefix; }) === index; });
  }

  function activeAccountKeys(access, account) {
    return accountKeys(access, account).filter(keyIsActive);
  }

  function keyIsActive(key) {
    var status = String(key.status || '').toLowerCase();
    return status !== 'revoked' && status !== 'expired' && status !== 'disabled';
  }

  function renderAccounts(access, admin) {
    var list = q('#service-accounts-list');
    if (!list) return;
    removeChildren(list);
    if (access.unavailable) { list.appendChild(create('div', 'list-empty', 'Access metadata could not be loaded; the service-account inventory is unknown.')); return; }
    if (!state.live || !access.serviceAccounts.length) { list.appendChild(create('div', 'list-empty', 'No service-account records supplied.')); return; }
    access.serviceAccounts.forEach(function (account) {
      var keys = accountKeys(access, account);
      var activeKeys = activeAccountKeys(access, account);
      var card = create('div', 'account-card');
      var identity = create('div'); identity.appendChild(create('strong', '', account.name));
      var meta = account.description || ('Created ' + formatDate(account.createdAt)); identity.appendChild(create('small', '', meta));
      card.appendChild(identity);
      var details = create('div', 'account-card__meta'); details.textContent = keys.length + ' key record' + (keys.length === 1 ? '' : 's') + ' · ' + activeKeys.length + ' active · ' + humanStatus(account.status); card.appendChild(details);
      var actions = create('div', 'account-card__actions');
      if (!activeKeys.length) {
        var first = create('button', 'row-action', keys.length ? 'Issue active key' : 'Issue first key'); first.type = 'button'; first.disabled = !admin; first.setAttribute('data-account-action', 'first'); first.setAttribute('data-account-id', account.id); actions.appendChild(first);
      } else {
        var overlap = create('button', 'row-action', 'Issue overlap key'); overlap.type = 'button'; overlap.disabled = !admin; overlap.setAttribute('data-account-action', 'overlap'); overlap.setAttribute('data-account-id', account.id); actions.appendChild(overlap);
      }
      card.appendChild(actions); list.appendChild(card);
    });
  }

  function renderKeys(access, admin) {
    var body = q('#access-keys-table tbody');
    if (!body) return;
    removeChildren(body);
    if (access.unavailable) { body.appendChild(emptyRow(8, 'Access metadata could not be loaded; the key inventory is unknown.')); return; }
    if (!state.live || !access.keys.length) { body.appendChild(emptyRow(8, 'No key records supplied.')); return; }
    access.keys.forEach(function (key) {
      var row = create('tr');
      var active = keyIsActive(key);
      appendCell(row, key.name);
      appendCell(row, key.prefix);
      appendCell(row, key.scopes.length ? key.scopes.join(', ') : 'Unknown');
      appendCell(row, formatDate(key.createdAt));
      appendCell(row, formatDate(key.expiresAt));
      appendCell(row, formatDate(key.lastUsedAt));
      appendCell(row, humanStatus(key.status));
      var actionsCell = create('td');
      var actions = create('div', 'row-actions');
      var overlap = create('button', 'row-action', 'Overlap'); overlap.type = 'button'; overlap.disabled = !admin || !active; overlap.setAttribute('data-key-action', 'overlap'); overlap.setAttribute('data-key-prefix', key.prefix); overlap.setAttribute('data-key-account', key.serviceAccountId); actions.appendChild(overlap);
      var rotate = create('button', 'row-action', 'Rotate'); rotate.type = 'button'; rotate.disabled = !admin || !active; rotate.setAttribute('data-key-action', 'rotate'); rotate.setAttribute('data-key-prefix', key.prefix); rotate.setAttribute('data-key-account', key.serviceAccountId); actions.appendChild(rotate);
      var revoke = create('button', 'row-action', 'Revoke'); revoke.type = 'button'; revoke.disabled = !admin || String(key.status).toLowerCase() === 'revoked'; revoke.setAttribute('data-key-action', 'revoke'); revoke.setAttribute('data-key-prefix', key.prefix); revoke.setAttribute('data-key-account', key.serviceAccountId); actions.appendChild(revoke);
      actionsCell.appendChild(actions); row.appendChild(actionsCell); body.appendChild(row);
    });
  }

  function renderAll() {
    renderMe(state.me || normalizeMe({}));
    renderData(state.dashboard || blankDashboard('No connected dashboard snapshot was supplied.'));
    renderModels(state.catalogue || unavailableCatalogue('No connected model catalogue was supplied.'));
    renderEndpoints(state.endpoints || unavailableEndpoints('No connected endpoint inventory was supplied.'));
    renderBilling(state.billing || unavailableBilling('No connected billing snapshot was supplied.'));
    renderConfigurator();
    renderRequestProgress(state.requestProgress);
    renderAccess(state.access);
  }

  function clearScopedDOM() {
    state.dashboard = null;
    state.access = null;
    state.catalogue = null;
    state.endpoints = null;
    state.modelDetail = null;
    state.endpointDetail = null;
    state.requestProgress = null;
    state.billing = null;
    state.pageError = null;
    var defaults = {
      'source.label': 'Loading connected snapshot', 'source.detail': 'Refreshing the selected membership/session.', 'source.kind': 'Loading', 'source.freshness': 'Unknown', 'source.finality': 'Unknown', 'source.asOfText': 'Not available', 'export.status': 'Checking export availability for this snapshot.',
      'scope.organization': 'Loading', 'scope.project': 'Loading', 'scope.environment': 'Loading', 'scope.projectEnvironment': 'Loading', 'scope.detail': 'Refreshing membership/session scope.', 'identity.role': 'Loading',
      'route.stateLabel': 'Loading', 'route.statusLabel': 'Loading route evidence', 'route.statusDetail': 'Refreshing the selected route and service plan.', 'route.attentionTitle': 'Loading', 'route.attentionDetail': 'Refreshing route evidence.', 'route.freshness': 'Loading', 'route.evidenceSource': 'Loading', 'route.evidenceDetail': 'Refreshing route evidence.', 'route.executionClass': 'Loading', 'route.capacityMode': 'Loading', 'route.boundaryHeadline': 'Loading selected route boundary', 'route.boundaryDetail': 'Refreshing execution and capacity from the selected route and service plan.', 'route.capacityHeadline': 'Loading execution and capacity.', 'route.lastSuccess': 'Loading', 'route.lastObservation': 'Loading', 'route.modelAlias': 'Loading', 'route.selectionHelp': 'Refreshing route options.',
      'usage.requests': 'Loading', 'usage.successful': 'Loading', 'usage.outcomes': 'Loading', 'usage.tokensFinality': 'Loading', 'usage.tokenDetail': 'Loading', 'usage.p95': 'Loading', 'usage.throughput': 'Loading', 'usage.throughputHelp': 'Refreshing throughput_rps.', 'usage.concurrency': 'Loading', 'usage.concurrencyHelp': 'Refreshing peak_concurrency.', 'usage.stateLabel': 'Loading usage', 'usage.stateDetail': 'Refreshing usage ledger.', 'allocation.shared': 'Loading', 'allocation.dedicated': 'Loading', 'allocation.source': 'Loading', 'allocation.finality': 'Loading', 'allocation.contextDetail': 'Refreshing allowance and allocation context.', 'attribution.serviceAccount': 'Loading', 'trend.unit': 'Loading', 'access.source': 'Loading access', 'access.scopeHelp': 'Refreshing backend-allowed scopes.', 'docs.endpoint': 'Endpoint unavailable — safe gateway URL not supplied'
    };
    Object.keys(defaults).forEach(function (key) { bind(key, defaults[key]); });
    bindTime('source.asOf', null);
    setState(q('#global-state'), 'loading');
    setState(q('#usage-state'), 'loading');
    all('[data-empty-message]').forEach(function (node) { node.hidden = true; });
    var tables = [['#service-attribution-body', 4], ['#trend-table tbody', 5], ['#model-breakdown-table tbody', 5], ['#project-breakdown-table tbody', 4], ['#recent-requests-table tbody', 9], ['#access-keys-table tbody', 8], ['#models-table tbody', 7], ['#model-profiles-table tbody', 6], ['#endpoints-table tbody', 8], ['#billing-invoices-table tbody', 6]];
    tables.forEach(function (item) { var body = q(item[0]); if (body) { removeChildren(body); body.appendChild(emptyRow(item[1], 'Loading connected data.')); } });
    var accounts = q('#service-accounts-list'); if (accounts) { removeChildren(accounts); accounts.appendChild(create('div', 'list-empty', 'Loading connected data.')); }
    bind('models.stateLabel', 'Loading model catalogue'); bind('models.stateDetail', 'Refreshing connected model releases.'); bind('models.source', 'Loading');
    bind('endpoints.stateLabel', 'Loading endpoints'); bind('endpoints.stateDetail', 'Refreshing connected endpoint inventory.'); bind('endpoints.source', 'Loading');
    bind('billing.stateLabel', 'Loading billing'); bind('billing.stateDetail', 'Refreshing billing state.'); bind('billing.source', 'Loading');
    optionList(q('#route-model-select'), [], 'Loading model aliases', '');
    optionList(q('#usage-model'), [], 'Loading model aliases', '');
    var svg = q('#usage-chart');
    if (svg) { removeChildren(q('.chart-grid', svg)); removeChildren(q('.chart-points', svg)); q('.chart-line', svg).setAttribute('points', ''); }
    setHidden(q('#chart-empty'), false);
  }

  function authFailure() {
    renderGlobal({ label: 'Sign-in required', detail: 'This membership/session is not authenticated. Redirecting to the gentle sign-in page.', kind: 'Authentication', freshness: 'Unavailable', finality: 'Unknown', asOf: null }, 'error');
    window.setTimeout(function () { window.location.assign('/login'); }, 250);
  }

  async function loadResourceSnapshot(loadGeneration) {
    if (!state.live) return;
    var results = await Promise.all([request('/api/portal/catalogue/models'), request('/api/portal/endpoints'), request('/api/portal/billing')]);
    if (loadGeneration && loadGeneration !== state.loadGeneration) return;
    var catalogueResult = results[0];
    var endpointsResult = results[1];
    var billingResult = results[2];
    if (catalogueResult.status === 401 || endpointsResult.status === 401 || billingResult.status === 401) { authFailure(); return; }
    state.catalogue = catalogueResult.ok ? normalizeCatalogue(catalogueResult.data) : unavailableCatalogue(catalogueResult.status === 0 ? 'The model catalogue could not be reached.' : 'The model catalogue endpoint did not return a usable snapshot.');
    state.endpoints = endpointsResult.ok ? normalizeEndpoints(endpointsResult.data) : unavailableEndpoints(endpointsResult.status === 0 ? 'The endpoint inventory could not be reached.' : 'The endpoint inventory endpoint did not return a usable snapshot.');
    state.billing = billingResult.ok ? normalizeBilling(billingResult.data) : unavailableBilling(billingResult.status === 403 ? 'Billing is not permitted for this membership/session.' : billingResult.status === 0 ? 'The billing service could not be reached. Retry when the service is available.' : 'The billing endpoint did not return a usable snapshot.');
    await loadResourceDetail();
  }

  async function loadResourceDetail() {
    var generation = ++state.resourceGeneration;
    var resource = state.resource || routeFromLocation();
    state.resource = resource;
    if (!state.live || !resource) return;
    if (resource.kind === 'model-detail') {
      var currentModel = state.catalogue && state.catalogue.models.filter(function (model) { return model.slug === resource.slug; })[0];
      if (currentModel && currentModel.profiles.length) state.modelDetail = currentModel;
      else {
        var modelResult = await request('/api/portal/catalogue/models/' + encodeURIComponent(resource.slug));
        if (generation !== state.resourceGeneration) return;
        if (modelResult.ok) { var modelRoot = unwrap(modelResult.data); state.modelDetail = normalizeModel(firstValue(modelRoot, ['model']) || modelRoot); if (state.catalogue) state.catalogue.models = state.catalogue.models.concat([state.modelDetail]).filter(function (model, index, list) { return list.findIndex(function (item) { return item.slug === model.slug; }) === index; }); }
        else state.modelDetail = null;
      }
    } else if (resource.kind === 'endpoint-detail') {
      var currentEndpoint = state.endpoints && state.endpoints.endpoints.filter(function (endpoint) { return endpoint.id === resource.id; })[0];
      if (currentEndpoint) state.endpointDetail = currentEndpoint;
      else {
        var endpointResult = await request('/api/portal/endpoints/' + encodeURIComponent(resource.id));
        if (generation !== state.resourceGeneration) return;
        if (endpointResult.ok) { var endpointRoot = unwrap(endpointResult.data); state.endpointDetail = normalizeEndpoint(firstValue(endpointRoot, ['endpoint']) || endpointRoot); }
        else state.endpointDetail = null;
      }
    } else if (resource.kind === 'request') {
      await loadRequestProgress(resource.id, false, generation);
    } else if (resource.kind === 'config' && resource.draftId) {
      var configurationResult = await request('/api/portal/endpoint-configurations/' + encodeURIComponent(resource.draftId));
      if (generation !== state.resourceGeneration) return;
      if (configurationResult.status === 401) { authFailure(); return; }
      if (configurationResult.ok) restoreConfiguration(normalizeConfiguration(configurationResult.data));
      else setConfigError(configurationResult.status === 404 ? 'This saved draft was not found in the current membership/session.' : 'The saved draft could not be restored. No endpoint state changed.');
    }
  }

  async function loadRequestProgress(id, preserveLocation, expectedGeneration) {
    if (!state.live || !id) { state.requestProgress = null; return null; }
    var result = await request('/api/portal/deployment-requests/' + encodeURIComponent(id));
    if (expectedGeneration && expectedGeneration !== state.resourceGeneration) return null;
    if (result.status === 401) { authFailure(); return null; }
    if (!result.ok) { state.requestProgress = null; return null; }
    state.requestProgress = normalizeRequestProgress(result.data);
    if (state.requestProgress.quoteId) {
      var quoteResult = await request('/api/portal/deployment-quotes/' + encodeURIComponent(state.requestProgress.quoteId));
      if (expectedGeneration && expectedGeneration !== state.resourceGeneration) return null;
      if (quoteResult.ok) {
        var quoteRoot = unwrap(quoteResult.data);
        var quote = firstValue(quoteRoot, ['quote']) || quoteRoot;
        state.requestProgress.raw.quote = quote;
        state.requestProgress.commercial = normalizeProgressRail(quote, state.requestProgress.commercial.detail);
      }
    }
    if (!preserveLocation) renderRequestProgress(state.requestProgress);
    return state.requestProgress;
  }

  function requestRecentAuthentication(resume, label) {
    state.pendingCommercialAction = { resume: resume, label: label || 'commercial action' };
    var form = q('#reauth-form'); if (form) form.reset();
    var error = q('#reauth-error'); if (error) { error.hidden = true; error.textContent = ''; }
    openDialog(q('#reauth-dialog'));
    window.requestAnimationFrame(function () { var password = q('#reauth-password'); if (password) password.focus(); });
  }

  async function submitReauthentication(event) {
    event.preventDefault();
    var password = q('#reauth-password');
    var error = q('#reauth-error');
    if (error) { error.hidden = true; error.textContent = ''; }
    if (!password || !password.value) { formError(error, 'Enter your portal password. An inference API key is not a portal password.'); return; }
    var form = q('#reauth-form');
    var submit = form && form.querySelector('button[type="submit"]');
    if (submit) { submit.disabled = true; submit.textContent = 'Confirming…'; }
    var result = await request('/api/portal/reauthenticate', { method: 'POST', body: { password: password.value } });
    password.value = '';
    if (submit) { submit.disabled = false; submit.textContent = 'Confirm and continue'; }
    if (!result.ok) { formError(error, result.status === 401 ? 'That portal password was not accepted.' : responseMessage(result, 'Password confirmation is temporarily unavailable. No commercial action was performed.')); return; }
    var pending = state.pendingCommercialAction;
    state.pendingCommercialAction = null;
    closeDialog(q('#reauth-dialog'));
    showToast('Password confirmed. Continuing the requested action.');
    if (pending && typeof pending.resume === 'function') await pending.resume(true);
  }

  async function acceptQuote(afterReauthentication) {
    var progress = state.requestProgress;
    if (!progress || !progress.quoteId) { showToast('No quote reference is available.'); return; }
    var operation = 'quote-accept-' + progress.quoteId;
    var result = await request('/api/portal/deployment-quotes/' + encodeURIComponent(progress.quoteId) + '/accept', { method: 'POST', body: {}, idempotency: operation });
    if (recentAuthenticationRequired(result) && !afterReauthentication) { requestRecentAuthentication(acceptQuote, 'quote acceptance'); return; }
    if (!result.ok) { if (definitiveResult(result)) clearOperationKey(operation); showToast(result.status === 403 ? 'Your role cannot accept this quote.' : responseMessage(result, 'The quote could not be accepted; no commercial state was changed.')); return; }
    clearOperationKey(operation);
    showToast('Quote acceptance was recorded. Payment and provisioning remain separate states.');
    await loadRequestProgress(progress.id, false);
  }

  async function startCheckout(afterReauthentication) {
    var progress = state.requestProgress;
    if (!progress || !progress.paymentRequirementId) { showToast('No payment requirement is available.'); return; }
    var operation = 'checkout-' + progress.paymentRequirementId;
    var result = await request('/api/portal/payment-requirements/' + encodeURIComponent(progress.paymentRequirementId) + '/checkout-session', { method: 'POST', body: {}, idempotency: operation });
    if (recentAuthenticationRequired(result) && !afterReauthentication) { requestRecentAuthentication(startCheckout, 'hosted checkout'); return; }
    if (!result.ok) { if (definitiveResult(result)) clearOperationKey(operation); showToast(result.status === 403 ? 'Your role cannot start checkout.' : responseMessage(result, 'Hosted checkout could not be opened; payment state is unchanged.')); return; }
    clearOperationKey(operation);
    var root = unwrap(result.data);
    var url = normalizeHostedUrl(firstValue(root, ['url', 'checkout_url', 'hosted_url']));
    if (!url || !/^https:/.test(url)) { showToast('The backend did not return a safe hosted checkout URL. Payment state is unchanged.'); return; }
    window.location.assign(url);
  }

  async function manageBilling(afterReauthentication) {
    if (!state.live || !state.billing || state.billing.canManage !== true) { showToast('Billing management is unavailable for this membership/session.'); return; }
    var operation = 'billing-portal';
    var result = await request('/api/portal/billing/portal-session', { method: 'POST', body: {}, idempotency: operation });
    if (recentAuthenticationRequired(result) && !afterReauthentication) { requestRecentAuthentication(manageBilling, 'billing management'); return; }
    if (!result.ok) { if (definitiveResult(result)) clearOperationKey(operation); showToast(result.status === 403 ? 'Billing permission is required for this membership/session.' : responseMessage(result, 'The hosted billing portal could not be opened.')); return; }
    clearOperationKey(operation);
    var url = normalizeHostedUrl(firstValue(unwrap(result.data), ['url', 'portal_url', 'hosted_url']));
    if (!url || !/^https:/.test(url)) { showToast('The backend did not return a safe hosted billing URL.'); return; }
    window.location.assign(url);
  }

  function openCapacityDialog(endpointId) {
    var endpoints = state.endpoints && state.endpoints.endpoints || [];
    var endpoint = endpoints.filter(function (item) { return item.id === endpointId; })[0] || (state.endpointDetail && state.endpointDetail.id === endpointId ? state.endpointDetail : null);
    if (!endpoint || endpoint.mode !== 'Dedicated') { showToast('Capacity changes are available only for a connected dedicated endpoint.'); return; }
    state.capacityTarget = endpoint;
    var form = q('#capacity-form'); if (form) form.reset();
    var current = endpoint.capacityUnits;
    var units = q('#capacity-units'); if (units) units.value = String(current !== null && current !== undefined ? Math.min(128, current + 1) : 1);
    bindText('#capacity-context', endpoint.alias + ' currently reports ' + endpoint.capacity + '. Request a new total allocation; Alzette will review it before any commercial or infrastructure change.');
    var error = q('#capacity-error'); if (error) { error.hidden = true; error.textContent = ''; }
    openDialog(q('#capacity-dialog'));
  }

  async function submitCapacityRequest(event) {
    event.preventDefault();
    var endpoint = state.capacityTarget;
    var error = q('#capacity-error'); if (error) { error.hidden = true; error.textContent = ''; }
    if (!endpoint || endpoint.mode !== 'Dedicated') { formError(error, 'The dedicated endpoint context is no longer available.'); return; }
    var units = Number(q('#capacity-units') && q('#capacity-units').value);
    if (!Number.isInteger(units) || units < 1 || units > 128 || units === endpoint.capacityUnits) { formError(error, 'Choose a new total between 1 and 128 capacity units.'); return; }
    var workload = {};
    var useCase = q('#capacity-use-case') && q('#capacity-use-case').value.trim();
    var concurrency = Number(q('#capacity-concurrency') && q('#capacity-concurrency').value);
    var contextTokens = Number(q('#capacity-context-window') && q('#capacity-context-window').value);
    if (useCase) workload.use_case = useCase;
    if (concurrency) workload.expected_concurrency = concurrency;
    if (contextTokens) workload.expected_context_tokens = contextTokens;
    var operation = 'capacity-request-' + endpoint.id + '-' + units;
    var form = q('#capacity-form'); var submit = form && form.querySelector('button[type="submit"]');
    if (submit) { submit.disabled = true; submit.textContent = 'Submitting…'; }
    var result = await request('/api/portal/endpoints/' + encodeURIComponent(endpoint.id) + '/capacity-requests', { method: 'POST', body: { capacity_units: units, workload: workload }, idempotency: operation });
    if (submit) { submit.disabled = false; submit.textContent = 'Submit capacity request'; }
    if (!result.ok) { if (definitiveResult(result)) clearOperationKey(operation); formError(error, result.status === 403 ? 'Your role cannot request capacity for this endpoint.' : responseMessage(result, 'The capacity request could not be created. No allocation changed.')); return; }
    clearOperationKey(operation);
    var root = unwrap(result.data);
    var requestId = stringValue(firstValue(root, ['deployment_request.id', 'request_id', 'id']), '');
    closeDialog(q('#capacity-dialog'));
    state.capacityTarget = null;
    if (requestId) navigatePath('/app/endpoints/requests/' + encodeURIComponent(requestId), true);
    else showToast('Capacity request created, but no request reference was returned. No readiness claim was made.');
  }

  async function loadConnected(options) {
    if (!apiEnabled) return;
    var config = options || {};
    var generation = ++state.loadGeneration;
    state.live = true;
    state.loading = true;
    if (config.clear) clearScopedDOM();
    renderGlobal({ label: 'Loading connected snapshot', detail: 'Refreshing membership/session, route evidence, usage, and access.', kind: 'Loading', freshness: 'Unknown', finality: 'Unknown', asOf: null }, 'loading');
    var meResult = await request('/api/portal/me');
    if (generation !== state.loadGeneration) return;
    if (meResult.status === 401) { state.loading = false; authFailure(); return; }
    state.me = meResult.ok ? normalizeMe(meResult.data) : normalizeMe({});
    if (state.me.csrfToken) state.csrfToken = state.me.csrfToken;
    renderMe(state.me);
    var query = config.filters || state.filters || {};
    var dashboardResult = await request(queryPath('/api/portal/dashboard', query));
    if (generation !== state.loadGeneration) return;
    if (dashboardResult.status === 401) { state.loading = false; authFailure(); return; }
    if (dashboardResult.ok) {
      state.dashboard = normalizeDashboard(dashboardResult.data, state.me);
      state.pageError = null;
    } else {
      state.dashboard = blankDashboard(dashboardResult.status === 0 ? 'The connected dashboard could not be reached.' : 'The connected dashboard did not return a usable snapshot.');
      state.pageError = dashboardResult;
    }
    var accessResult = await request('/api/portal/access');
    if (generation !== state.loadGeneration) return;
    if (accessResult.status === 401) { state.loading = false; authFailure(); return; }
    state.access = accessResult.ok ? normalizeAccess(accessResult.data) : unavailableAccess(accessResult.status === 0 ? 'The Access API could not be reached. Inventory and permissions are unknown; all mutations are disabled.' : 'The Access API did not return a usable snapshot. Inventory and permissions are unknown; all mutations are disabled.');
    await loadResourceSnapshot(generation);
    if (generation !== state.loadGeneration) return;
    renderAll();
    state.loading = false;
  }

  async function refreshAccess(options) {
    if (!state.live) return;
    var preserveSecret = options && options.preserveSecret;
    var result = await request('/api/portal/access');
    if (result.status === 401) {
      if (preserveSecret) showToast('Access could not refresh. The one-time key remains available until this dialog closes.');
      else authFailure();
      return false;
    }
    if (!result.ok) {
      if (preserveSecret) showToast('Access could not refresh. The one-time key remains available until this dialog closes.');
      state.access = unavailableAccess('The Access API could not refresh. Inventory and permissions are unknown; all mutations are disabled.');
      renderAccess(state.access);
      return false;
    }
    state.access = result.ok ? normalizeAccess(result.data) : null;
    renderAccess(state.access);
    return true;
  }

  function renderFallback() {
    state.live = false;
    state.me = normalizeMe({ memberships: [] });
    state.dashboard = FALLBACK;
    state.catalogue = FALLBACK.catalogue;
    state.endpoints = FALLBACK.endpoints;
    state.billing = FALLBACK.billing;
    state.modelDetail = null;
    state.endpointDetail = null;
    state.requestProgress = null;
    state.access = unavailableAccess('This illustrative preview has no connected access metadata. Inventory and permissions are unknown.');
    renderAll();
    renderGlobal(FALLBACK.source, 'fallback');
  }

  function openDialog(dialog) {
    if (!dialog) return;
    if (typeof dialog.showModal === 'function') dialog.showModal();
    else dialog.setAttribute('open', '');
  }

  function closeDialog(dialog) {
    if (!dialog) return;
    if (typeof dialog.close === 'function') dialog.close();
    else dialog.removeAttribute('open');
  }

  function setKeyActionExplain(action) {
    all('[data-key-explain]').forEach(function (node) { node.hidden = node.getAttribute('data-key-explain') !== action; });
    bind('key.actionLabel', action === 'first' ? 'Issue first key' : action === 'rotate' ? 'Rotate key' : 'Issue overlap key');
    var copy = q('#key-dialog-copy');
    if (copy) copy.textContent = action === 'rotate' ? 'A replacement key is issued with its own name, scopes, and expiry. The old key remains active until you explicitly revoke it.' : 'The server will return plaintext only after a successful issue. Store it immediately; it cannot be recovered here.';
  }

  function setExpiryInputs() {
    var mode = q('#key-expiry-mode');
    var relative = q('#key-expiry');
    var exactWrap = q('#key-expiry-date-wrap');
    var exact = q('#key-expiry-date');
    var exactMode = mode && mode.value === 'exact';
    if (mode) mode.disabled = false;
    if (relative) relative.disabled = exactMode;
    if (exactWrap) exactWrap.hidden = !exactMode;
    if (exact) { exact.disabled = !exactMode; exact.required = exactMode; }
  }

  function openKeyDialog(action, accountId, key) {
    if (!canAdmin(state.access) || !state.live) { showToast('Administrator permission is required for key actions.'); return; }
    state.keyAction = action || 'overlap';
    state.keyTarget = key || null;
    var select = q('#key-service-account');
    if (select) {
      removeChildren(select);
      (state.access.serviceAccounts || []).forEach(function (account) {
        var option = create('option', '', account.name);
        option.value = account.id; option.selected = account.id === accountId; select.appendChild(option);
      });
    }
    var form = q('#key-form');
    if (form) form.reset();
    if (select && accountId) select.value = accountId;
    if (q('#key-name')) q('#key-name').value = key && key.name ? key.name + '-replacement' : '';
    setKeyActionExplain(state.keyAction);
    renderScopeOptions();
    renderExpiryPolicy();
    if (q('#key-name')) q('#key-name').disabled = false;
    if (q('#key-scope-fieldset')) q('#key-scope-fieldset').disabled = false;
    if (q('#key-expiry-mode')) q('#key-expiry-mode').disabled = false;
    if (q('#key-expiry')) q('#key-expiry').disabled = false;
    if (q('#key-expiry-date')) q('#key-expiry-date').disabled = false;
    if (q('#key-expiry-mode')) q('#key-expiry-mode').value = 'relative';
    setExpiryInputs();
    if (key && key.scopes && key.scopes.length) {
      all('input[name="key_scopes"]', q('#key-form')).forEach(function (input) { input.checked = key.scopes.indexOf(input.value) >= 0; });
    }
    if (key && key.expiresAt) {
      var expiryDate = new Date(key.expiresAt);
      if (!Number.isNaN(expiryDate.getTime()) && expiryDate.getTime() > Date.now() + 60 * 60 * 1000 && expiryDate.getTime() <= Date.now() + 365 * 24 * 60 * 60 * 1000) {
        q('#key-expiry-mode').value = 'exact';
        q('#key-expiry-date').value = expiryDate.toISOString().slice(0, 16);
        setExpiryInputs();
      }
    }
    if (q('#key-error')) { q('#key-error').hidden = true; q('#key-error').textContent = ''; }
    openDialog(q('#key-dialog'));
  }

  function extractSecret(payload) {
    var root = unwrap(payload);
    var key = firstValue(root, ['key']) || {};
    return firstValue(key, ['api_key']) || '';
  }

  function formError(node, message) {
    if (!node) return;
    node.textContent = message;
    node.hidden = false;
  }

  function expiryValue() {
    var mode = q('#key-expiry-mode');
    var exact = q('#key-expiry-date');
    var relative = q('#key-expiry');
    var now = Date.now();
    var min = now + 60 * 60 * 1000;
    var max = now + 365 * 24 * 60 * 60 * 1000;
    var date;
    if (mode && mode.value === 'exact') {
      date = exact && exact.value ? new Date(exact.value) : null;
    } else {
      var match = relative && String(relative.value).match(/^(\d+)d$/);
      var days = match ? Number(match[1]) : 90;
      date = new Date(now + days * 24 * 60 * 60 * 1000);
    }
    if (!date || Number.isNaN(date.getTime())) return { error: 'Choose an expiry date/time.' };
    if (date.getTime() < min) return { error: 'Expiry must be at least one hour from now.' };
    if (date.getTime() > max) return { error: 'Expiry must be no more than 365 days from now.' };
    return { value: date.toISOString() };
  }

  async function submitKeyForm(event) {
    event.preventDefault();
    var form = q('#key-form');
    var error = q('#key-error');
    if (error) { error.hidden = true; error.textContent = ''; }
    var accountId = q('#key-service-account') && q('#key-service-account').value;
    var name = q('#key-name') && q('#key-name').value.trim();
    var scopes = all('input[name="key_scopes"]:checked', form).map(function (input) { return input.value; });
    var expiry = null;
    if (!name) { formError(error, 'Key name is required.'); return; }
    if (!scopes.length) { formError(error, 'Select at least one least-privilege scope.'); return; }
    if (state.live && (!state.access || !state.access.allowedScopes.length)) { formError(error, 'The backend did not supply allowed scopes for this membership/session.'); return; }
    expiry = expiryValue();
    if (expiry.error) { formError(error, expiry.error); return; }
    if (!accountId) { formError(error, 'Choose a service account.'); return; }
    if (state.keyAction === 'rotate' && (!state.keyTarget || !state.keyTarget.prefix)) { formError(error, 'Choose the key to rotate.'); return; }
    var submit = form.querySelector('button[type="submit"]');
    if (submit) { submit.disabled = true; submit.textContent = state.keyAction === 'rotate' ? 'Rotating…' : 'Issuing…'; }
    var endpoint = state.keyAction === 'rotate' ? '/api/portal/keys/rotate' : '/api/portal/keys/issue';
    var body = { service_account_id: accountId, name: name, scopes: scopes, expires_at: expiry.value };
    if (state.keyAction === 'rotate') body.rotated_from_prefix = state.keyTarget.prefix;
    var operation = ['key', state.keyAction, accountId, name, scopes.slice().sort().join(','), expiry.value, body.rotated_from_prefix || ''].join(':');
    var result = await request(endpoint, { method: 'POST', body: body, idempotency: operation });
    if (submit) { submit.disabled = false; submit.textContent = 'Continue'; }
    if (!result.ok) {
      if (definitiveResult(result)) clearOperationKey(operation);
      var message = result.status === 403
        ? 'You can view this area, but your role cannot perform this action.'
        : result.status === 409
          ? 'A key with this name already exists. If an earlier one-time reveal was interrupted, refresh Access and issue a new key with a distinct name; plaintext cannot be recovered.'
          : 'The key action could not be completed. No plaintext key was shown.';
      formError(error, message);
      return;
    }
    clearOperationKey(operation);
    state.oneTimeSecret = String(extractSecret(result.data) || '');
    closeDialog(q('#key-dialog'));
    if (state.oneTimeSecret) {
      q('#secret-value').textContent = state.oneTimeSecret;
      openDialog(q('#secret-dialog'));
    } else {
      showToast('Key action completed, but the server returned no plaintext key. No secret was shown.');
    }
    try {
      await refreshAccess({ preserveSecret: !!state.oneTimeSecret });
    } catch (refreshError) {
      if (state.oneTimeSecret) showToast('Access could not refresh. The one-time key remains available until this dialog closes.');
    }
  }

  async function createServiceAccount(event) {
    event.preventDefault();
    var form = q('#service-account-form');
    var error = q('#service-account-error');
    if (error) { error.hidden = true; error.textContent = ''; }
    var name = q('#service-account-name') && q('#service-account-name').value.trim();
    if (!name) { formError(error, 'Service-account name is required.'); return; }
    var submit = form.querySelector('button[type="submit"]');
    if (submit) { submit.disabled = true; submit.textContent = 'Creating…'; }
    var operation = 'service-account-create:' + name;
    var result = await request('/api/portal/service-accounts', { method: 'POST', body: { name: name }, idempotency: operation });
    if (submit) { submit.disabled = false; submit.textContent = 'Create account'; }
    if (!result.ok) { if (definitiveResult(result)) clearOperationKey(operation); formError(error, result.status === 403 ? 'You can view this area, but your role cannot perform this action.' : 'The service account could not be created.'); return; }
    clearOperationKey(operation);
    var created = normalizeServiceAccount(firstValue(unwrap(result.data), ['service_account', 'serviceAccount']) || unwrap(result.data));
    closeDialog(q('#service-account-dialog'));
    await refreshAccess();
    var createdAccount = (state.access && state.access.serviceAccounts || []).filter(function (account) { return account.id === created.id || account.name === created.name || account.name === name; })[0];
    if (createdAccount) {
      showToast('Service account created. Review scopes and expiry before issuing its first key.');
      openKeyDialog('first', createdAccount.id);
    } else {
      showToast('Service account created. Refresh Access to issue its first key.');
    }
  }

  async function revokeKey(prefix, accountId) {
    if (!canAdmin(state.access) || !state.live) { showToast('Administrator permission is required for key actions.'); return; }
    if (!window.confirm('Revoke ' + prefix + '? This stops the key and cannot be undone.')) return;
    var operation = 'key-revoke:' + prefix;
    var result = await request('/api/portal/keys/revoke', { method: 'POST', body: { prefix: prefix }, idempotency: operation });
    if (!result.ok) { if (definitiveResult(result)) clearOperationKey(operation); showToast(result.status === 403 ? 'Administrator permission is required.' : 'The key could not be revoked.'); return; }
    clearOperationKey(operation);
    showToast('Key revoked.');
    await refreshAccess();
  }

  async function switchMembership(event) {
    event.preventDefault();
    var select = q('#membership-selector');
    var error = q('#context-error');
    if (error) { error.hidden = true; error.textContent = ''; }
    if (!select || !select.value) { formError(error, 'Choose a membership/session.'); return; }
    if (!state.csrfToken) { formError(error, 'This session did not supply a CSRF token; context cannot be changed safely.'); return; }
    var submit = q('#context-submit');
    if (submit) { submit.disabled = true; submit.textContent = 'Switching…'; }
    var operation = 'context-switch:' + select.value;
    var result = await request('/api/portal/context', { method: 'POST', body: { membership_id: select.value }, idempotency: operation });
    if (submit) { submit.disabled = false; submit.textContent = 'Use membership'; }
    if (!result.ok) { if (definitiveResult(result)) clearOperationKey(operation); formError(error, result.status === 403 ? 'This membership/session cannot be selected.' : 'Context could not be changed safely.'); return; }
    clearOperationKey(operation);
    var response = unwrap(result.data);
    state.csrfToken = stringValue(firstValue(response, ['csrf_token', 'csrfToken', 'csrf']), state.csrfToken);
    closeDialog(q('#context-dialog'));
    clearScopedDOM();
    await loadConnected({ clear: false, filters: state.filters });
    navigatePath(window.location.pathname + window.location.search, false);
  }

  function clearKeySecret() {
    state.oneTimeSecret = '';
    var secret = q('#secret-value');
    if (secret) secret.textContent = '';
  }

  function setupNavigation() {
    doc.addEventListener('click', function (event) {
      var link = event.target.closest ? event.target.closest('[data-view-link]') : null;
      if (!link) return;
      var href = link.getAttribute('href') || '';
      if (href.indexOf('/app/') !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return;
      event.preventDefault();
      navigatePath(href, true);
      closeMenu(false);
    });
    window.addEventListener('popstate', function () { navigatePath(window.location.pathname + window.location.search, false); });
  }

  function safeDecode(value) {
    try { return decodeURIComponent(String(value || '')); } catch (error) { return ''; }
  }

  function routeFromPath(pathname, search, hash) {
    var path = String(pathname || '').replace(/\/+$/, '') || '/';
    var match;
    if ((match = path.match(/^\/app\/models\/([^/]+)$/))) { var modelSlug = safeDecode(match[1]); return modelSlug ? { view: 'models', kind: 'model-detail', slug: modelSlug } : { view: 'models', kind: 'model-list' }; }
    if (path === '/app/models') return { view: 'models', kind: 'model-list' };
    if (path === '/app/endpoints/new') {
      var configQuery = new URLSearchParams(search || '');
      return { view: 'endpoints', kind: 'config', modelSlug: configQuery.get('model') || '', mode: configQuery.get('mode') || '', profileId: configQuery.get('profile') || '', draftId: configQuery.get('draft') || '', operationId: configQuery.get('operation') || '' };
    }
    if ((match = path.match(/^\/app\/endpoints\/requests\/([^/]+)$/))) { var requestId = safeDecode(match[1]); return requestId ? { view: 'endpoints', kind: 'request', id: requestId } : { view: 'endpoints', kind: 'endpoint-list' }; }
    if ((match = path.match(/^\/app\/endpoints\/([^/]+)$/))) { var endpointId = safeDecode(match[1]); return endpointId ? { view: 'endpoints', kind: 'endpoint-detail', id: endpointId } : { view: 'endpoints', kind: 'endpoint-list' }; }
    if (path === '/app/endpoints' || path === '/app/routes') return { view: 'endpoints', kind: 'endpoint-list', legacy: path === '/app/routes' };
    if (path === '/app/billing') return { view: 'billing', kind: 'billing' };
    if (path === '/app/usage') { var usageQuery = new URLSearchParams(search || ''); return { view: 'usage', kind: 'view', model: usageQuery.get('model') || '', from: usageQuery.get('from') || '', to: usageQuery.get('to') || '' }; }
    var simple = path.match(/^\/app\/(overview|access|docs)$/);
    if (simple) return { view: simple[1], kind: 'view' };
    var legacy = String(hash || '').replace(/^#/, '');
    if (legacy === 'routes') legacy = 'endpoints';
    if (paths[legacy]) return { view: legacy, kind: legacy === 'models' ? 'model-list' : legacy === 'endpoints' ? 'endpoint-list' : legacy };
    return { view: 'overview', kind: 'view' };
  }

  function routeFromLocation() {
    return routeFromPath(window.location.pathname, window.location.search, window.location.hash);
  }

  function viewFromLocation() {
    return routeFromLocation().view;
  }

  function navigate(view, push) {
    if (paths[view]) navigatePath(paths[view], push);
    else navigatePath(view, push);
  }

  function navigatePath(path, push) {
    if (!hasPortal) return;
    var url = new URL(path, window.location.origin);
    if (push && window.history && window.history.pushState) {
      window.history.pushState({}, '', url.pathname + url.search);
      window.scrollTo(0, 0);
    }
    var previousResource = state.resource;
    state.resource = routeFromPath(url.pathname, url.search, url.hash);
    if (state.resource.kind === 'request' && (!previousResource || previousResource.kind !== 'request' || previousResource.id !== state.resource.id)) state.requestProgress = null;
    if (state.resource.kind === 'config') {
      var newConfiguration = !previousResource || previousResource.kind !== 'config' || previousResource.draftId !== state.resource.draftId || (push && !state.resource.draftId);
      if (newConfiguration) {
        state.configurator.values = {};
        state.configurator.draftLoaded = false;
      }
      state.configurator.modelSlug = state.resource.modelSlug || '';
      state.configurator.mode = state.resource.mode || '';
      state.configurator.profileId = state.resource.profileId || '';
      state.configurator.draftId = state.resource.draftId || '';
      if (!state.configurator.draftId) state.configurator.draftLoaded = false;
      state.configurator.operationId = state.resource.operationId || '';
      state.configurator.step = 1;
      if (state.configurator.modelSlug) state.configurator.step = state.configurator.mode ? (state.configurator.profileId ? 5 : 2) : 1;
    }
    var usageModelChanged = false;
    if (state.resource.view === 'usage') {
      var previousModel = state.filters.model || '';
      state.filters.model = state.resource.model || '';
      state.filters.from = state.resource.from || state.filters.from || '';
      state.filters.to = state.resource.to || state.filters.to || '';
      state.routeSelection = state.filters.model;
      usageModelChanged = previousModel !== state.filters.model;
    }
    var view = state.resource.view;
    all('section[data-view]').forEach(function (section) { section.hidden = section.getAttribute('data-view') !== view; });
    all('[data-view-link]').forEach(function (link) {
      var href = link.getAttribute('href') || '';
      var linkPath = href.indexOf('/app/') === 0 ? href.split('?')[0].replace(/\/$/, '') : href;
      var active = (view === 'models' && linkPath === paths.models) || (view === 'endpoints' && linkPath === paths.endpoints) || linkPath === paths[view];
      link.setAttribute('aria-current', active ? 'page' : 'false');
    });
    doc.body.setAttribute('data-view', view);
    var heading = q('section[data-view="' + view + '"] h1');
    if (heading) doc.title = heading.textContent + ' — Alzette Systems';
    renderModels(state.catalogue || unavailableCatalogue());
    renderEndpoints(state.endpoints || unavailableEndpoints());
    renderBilling(state.billing || unavailableBilling());
    renderConfigurator();
    renderRequestProgress(state.requestProgress);
    if (state.live && state.me && (view === 'models' || view === 'endpoints' || view === 'billing')) loadResourceDetail().then(function () { renderModels(state.catalogue || unavailableCatalogue()); renderEndpoints(state.endpoints || unavailableEndpoints()); renderBilling(state.billing || unavailableBilling()); renderConfigurator(); renderRequestProgress(state.requestProgress); });
    if (state.live && state.dashboard && view === 'usage' && usageModelChanged) loadConnected({ clear: true, filters: state.filters });
  }

  function mobileMenuLayout() {
    if (typeof window.matchMedia === 'function') return window.matchMedia('(max-width: 900px)').matches;
    return window.innerWidth <= 900;
  }

  function syncMenuAccessibility(open) {
    var rail = q('#portal-nav');
    var scrim = q('#nav-scrim');
    var mobile = mobileMenuLayout();
    if (rail) {
      rail.inert = mobile && !open;
      if (mobile) rail.setAttribute('aria-hidden', open ? 'false' : 'true');
      else rail.removeAttribute('aria-hidden');
    }
    if (scrim) scrim.hidden = !(mobile && open);
  }

  function closeMenu(restoreFocus) {
    var wasOpen = doc.body.classList.contains('nav-open');
    doc.body.classList.remove('nav-open');
    var toggle = q('#menu-toggle');
    if (toggle) toggle.setAttribute('aria-expanded', 'false');
    syncMenuAccessibility(false);
    if (wasOpen && restoreFocus !== false && toggle) toggle.focus();
  }

  function openMenu() {
    if (!mobileMenuLayout()) return;
    doc.body.classList.add('nav-open');
    var toggle = q('#menu-toggle');
    if (toggle) toggle.setAttribute('aria-expanded', 'true');
    syncMenuAccessibility(true);
    var close = q('#menu-close');
    if (close) window.requestAnimationFrame(function () { close.focus(); });
  }

  function setupMenu() {
    var toggle = q('#menu-toggle');
    if (!toggle) return;
    toggle.addEventListener('click', function () {
      var open = !doc.body.classList.contains('nav-open');
      if (open) openMenu();
      else closeMenu(true);
    });
    var close = q('#menu-close');
    if (close) close.addEventListener('click', function () { closeMenu(true); });
    var scrim = q('#nav-scrim');
    if (scrim) scrim.addEventListener('click', function () { closeMenu(true); });
    doc.addEventListener('keydown', function (event) { if (event.key === 'Escape' && doc.body.classList.contains('nav-open')) closeMenu(true); });
    window.addEventListener('resize', function () {
      if (!mobileMenuLayout()) {
        doc.body.classList.remove('nav-open');
        toggle.setAttribute('aria-expanded', 'false');
      }
      syncMenuAccessibility(doc.body.classList.contains('nav-open'));
    });
    syncMenuAccessibility(false);
  }

  function setupDialogs() {
    var contextButton = q('#context-button');
    if (contextButton) contextButton.addEventListener('click', function () { renderMembershipSelector(state.me || normalizeMe({})); openDialog(q('#context-dialog')); });
    all('[data-dialog-close]').forEach(function (button) { button.addEventListener('click', function () {
      var dialog = button.closest('dialog');
      if (dialog && dialog.id === 'reauth-dialog') { state.pendingCommercialAction = null; var password = q('#reauth-password'); if (password) password.value = ''; }
      if (dialog && dialog.id === 'capacity-dialog') state.capacityTarget = null;
      closeDialog(dialog);
    }); });
    var contextForm = q('#context-form');
    if (contextForm) contextForm.addEventListener('submit', function (event) { if (q('#context-submit') && !q('#context-submit').hidden) switchMembership(event); });
    var serviceOpen = q('#service-account-open');
    if (serviceOpen) serviceOpen.addEventListener('click', function () { if (canAdmin(state.access) && state.live) openDialog(q('#service-account-dialog')); else showToast('Administrator permission is required to create a service account.'); });
    var serviceForm = q('#service-account-form');
    if (serviceForm) serviceForm.addEventListener('submit', createServiceAccount);
    var keyForm = q('#key-form');
    if (keyForm) keyForm.addEventListener('submit', submitKeyForm);
    var reauthForm = q('#reauth-form');
    if (reauthForm) reauthForm.addEventListener('submit', submitReauthentication);
    var capacityForm = q('#capacity-form');
    if (capacityForm) capacityForm.addEventListener('submit', submitCapacityRequest);
    var expiryMode = q('#key-expiry-mode');
    if (expiryMode) expiryMode.addEventListener('change', setExpiryInputs);
    var secretDialog = q('#secret-dialog');
    if (secretDialog) secretDialog.addEventListener('close', clearKeySecret);
    var secretCopy = q('#secret-copy');
    if (secretCopy) secretCopy.addEventListener('click', function () { copyText(state.oneTimeSecret, 'Key copied. Store it in your secret manager.'); });
  }

  function copyText(value, success) {
    if (!value) return;
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(value).then(function () { showToast(success); }, function () { showToast('Copy was blocked; select the value manually.'); });
    } else {
      showToast('Copy is unavailable; select the value manually.');
    }
  }

  async function downloadExport(format) {
    if (!state.live) { showToast('Export becomes available when an authenticated usage snapshot is connected.'); return; }
    var meta = state.dashboard && state.dashboard.exportMeta || { available: false, formats: [] };
    var normalizedFormat = String(format || '').toLowerCase();
    if (meta.available !== true || !Array.isArray(meta.formats) || meta.formats.indexOf(normalizedFormat) < 0) {
      showToast('This format is unavailable until the backend reports a final exportable usage snapshot.');
      return;
    }
    if (!state.csrfToken) { showToast('Export is unavailable because this session did not supply a CSRF token.'); return; }
    var path = queryPath('/api/portal/usage/export', Object.assign({}, state.filters, { format: normalizedFormat }));
    try {
      var response = await fetch(path, { method: 'GET', credentials: 'same-origin', headers: { Accept: normalizedFormat === 'csv' ? 'text/csv' : 'application/json', 'X-CSRF-Token': state.csrfToken } });
      if (response.status === 401) { authFailure(); return; }
      if (!response.ok) { showToast('The ' + normalizedFormat.toUpperCase() + ' export could not be generated.'); return; }
      var blob = await response.blob();
      var disposition = response.headers.get('Content-Disposition') || '';
      var match = disposition.match(/filename="?([^";]+)"?/i);
      var filename = match ? match[1] : 'alzette-usage.' + normalizedFormat;
      var objectUrl = URL.createObjectURL(blob);
      var anchor = create('a');
      anchor.href = objectUrl; anchor.download = filename; anchor.hidden = true;
      doc.body.appendChild(anchor); anchor.click(); anchor.remove();
      window.setTimeout(function () { URL.revokeObjectURL(objectUrl); }, 1000);
      showToast(normalizedFormat.toUpperCase() + ' export downloaded.');
    } catch (error) {
      showToast('The export could not be downloaded.');
    }
  }

  function setupEvents() {
    doc.addEventListener('click', function (event) {
      var copy = event.target.closest ? event.target.closest('[data-copy-value]') : null;
      if (copy) { copyText(copy.getAttribute('data-copy-value'), 'Copied.'); return; }
      var copyTarget = event.target.closest ? event.target.closest('[data-copy-target]') : null;
      if (copyTarget) { var target = q('#' + copyTarget.getAttribute('data-copy-target')); if (target) copyText(target.textContent, 'Command copied.'); return; }
      var exportButton = event.target.closest ? event.target.closest('[data-export-format]') : null;
      if (exportButton) { event.preventDefault(); downloadExport(exportButton.getAttribute('data-export-format')); return; }
      var requestAction = event.target.closest ? event.target.closest('[data-request-action]') : null;
      if (requestAction) { event.preventDefault(); if (requestAction.getAttribute('data-request-action') === 'accept-quote') acceptQuote(); else if (requestAction.getAttribute('data-request-action') === 'checkout') startCheckout(); return; }
      if (event.target.closest && event.target.closest('#billing-manage')) { event.preventDefault(); manageBilling(); return; }
      if (event.target.closest && event.target.closest('#billing-retry')) { event.preventDefault(); loadConnected({ clear: true, filters: state.filters }); return; }
      var capacityAction = event.target.closest ? event.target.closest('[data-capacity-action]') : null;
      if (capacityAction) { event.preventDefault(); openCapacityDialog(capacityAction.getAttribute('data-capacity-action')); return; }
      var accountAction = event.target.closest ? event.target.closest('[data-account-action]') : null;
      if (accountAction) { openKeyDialog(accountAction.getAttribute('data-account-action'), accountAction.getAttribute('data-account-id')); return; }
      var keyAction = event.target.closest ? event.target.closest('[data-key-action]') : null;
      if (keyAction) {
        var action = keyAction.getAttribute('data-key-action');
        if (action === 'revoke') revokeKey(keyAction.getAttribute('data-key-prefix'), keyAction.getAttribute('data-key-account'));
        else {
          var targetPrefix = keyAction.getAttribute('data-key-prefix');
          var targetKey = (state.access && state.access.keys || []).filter(function (item) { return item.prefix === targetPrefix; })[0] || { name: targetPrefix, prefix: targetPrefix };
          openKeyDialog(action, keyAction.getAttribute('data-key-account'), targetKey);
        }
      }
    });
  }

  function setupModels() {
    var filters = q('#model-filters');
    if (!filters) return;
    var search = q('#model-search'); var mode = q('#model-mode-filter');
    if (search) search.addEventListener('input', function () { state.modelSearch = search.value; renderModels(state.catalogue || unavailableCatalogue()); });
    if (mode) mode.addEventListener('change', function () { state.modelModeFilter = mode.value; renderModels(state.catalogue || unavailableCatalogue()); });
    filters.addEventListener('submit', function (event) { event.preventDefault(); });
    filters.addEventListener('reset', function () { window.setTimeout(function () { state.modelSearch = ''; state.modelModeFilter = ''; renderModels(state.catalogue || unavailableCatalogue()); }, 0); });
  }

  function setupConfigurator() {
    var form = q('#endpoint-config-form');
    if (!form) return;
    function configurationChanged(persist) {
      if (!state.configurator.draftId) state.configurator.operationId = '';
      if (persist) persistConfiguratorURL(true);
    }
    var modelSelect = q('#config-model');
    if (modelSelect) modelSelect.addEventListener('change', function () { state.configurator.modelSlug = modelSelect.value; state.configurator.mode = ''; state.configurator.profileId = ''; var selected = (state.catalogue && state.catalogue.models || []).filter(function (item) { return item.slug === modelSelect.value; })[0]; state.configurator.values.model_slug = modelSelect.value; state.configurator.values.alias = selected ? selected.endpointAlias : ''; configurationChanged(true); renderConfigurator(); });
    all('input[name="deployment_mode"]').forEach(function (input) { input.addEventListener('change', function () { state.configurator.mode = input.value; state.configurator.profileId = ''; configurationChanged(true); renderConfigurator(); }); });
    var profile = q('#config-profile'); if (profile) profile.addEventListener('change', function () { state.configurator.profileId = profile.value; configurationChanged(true); renderConfigurator(); });
    ['config-alias', 'config-use-case', 'config-context', 'config-concurrency', 'config-latency', 'config-units'].forEach(function (id) { var node = q('#' + id); if (node) node.addEventListener('input', function () { captureConfiguratorValues(); configurationChanged(false); }); });
    var next = q('#config-next'); if (next) next.addEventListener('click', function () { var error = validateConfigStep(state.configurator.step); if (error) { setConfigError(error); return; } setConfigError(''); state.configurator.step = Math.min(6, state.configurator.step + 1); renderConfigurator(); });
    var back = q('#config-back'); if (back) back.addEventListener('click', function () { state.configurator.step = Math.max(1, state.configurator.step - 1); setConfigError(''); renderConfigurator(); });
    var save = q('#config-save'); if (save) save.addEventListener('click', function () { saveConfiguration(false); });
    form.addEventListener('submit', function (event) { event.preventDefault(); saveConfiguration(true); });
  }

  function setupUsage() {
    var form = q('#usage-filters');
    if (form) form.addEventListener('submit', function (event) {
      event.preventDefault();
      state.filters = { from: dateInputRFC3339(q('#usage-from') && q('#usage-from').value, false), to: dateInputRFC3339(q('#usage-to') && q('#usage-to').value, true), model: q('#usage-model') && q('#usage-model').value };
      state.routeSelection = state.filters.model || '';
      if (window.location.pathname === '/app/usage') window.history.replaceState({}, '', queryPath('/app/usage', state.filters));
      loadConnected({ clear: true, filters: state.filters });
    });
    var routeSelect = q('#route-model-select');
    if (routeSelect) routeSelect.addEventListener('change', function () {
      state.routeSelection = routeSelect.value;
      if (state.live) loadConnected({ clear: true, filters: Object.assign({}, state.filters, { model: state.routeSelection }) });
      else renderRoute(state.dashboard || FALLBACK);
    });
  }

  function setupSignout() {
    var button = q('#sign-out-button');
    if (!button) return;
    button.addEventListener('click', async function () {
      if (state.live) await request('/logout', { method: 'POST' });
      window.location.assign('/login');
    });
  }

  function setupLogin() {
    var form = q('#login-form');
    if (!form || !apiEnabled) return;
    form.addEventListener('submit', async function (event) {
      event.preventDefault();
      var button = form.querySelector('button[type="submit"]');
      var error = q('#login-error');
      if (error) { error.hidden = true; error.textContent = ''; }
      if (button) { button.disabled = true; button.textContent = 'Signing in…'; }
      try {
        var response = await fetch(form.getAttribute('action') || '/login', { method: 'POST', credentials: 'same-origin', body: new FormData(form), redirect: 'manual' });
        if (response.ok || response.type === 'opaqueredirect' || (response.status >= 300 && response.status < 400)) { window.location.assign('/app/overview'); return; }
        formError(error, 'That sign-in was not accepted. Check the portal credentials and try again.');
      } catch (requestError) {
        formError(error, 'The portal could not be reached. Try again on the trusted LAN connection.');
      }
      if (button) { button.disabled = false; button.textContent = 'Sign in →'; }
    });
  }

  function init() {
    showTransportNotice();
    setupLogin();
    if (!hasPortal) return;
    setupNavigation();
    setupMenu();
    setupDialogs();
    setupEvents();
    setupUsage();
    setupModels();
    setupConfigurator();
    setupSignout();
    navigatePath(window.location.pathname + window.location.search, false);
    if (state.live) loadConnected({ clear: false, filters: state.filters });
    else renderFallback();
  }

  if (doc.readyState === 'loading') doc.addEventListener('DOMContentLoaded', init);
  else init();
}());
