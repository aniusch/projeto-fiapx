// FIAP X — spike test: "Em caso de picos, o sistema não deve perder uma requisição."
//
// What it proves, end to end:
//   1. Under a sudden upload spike, the gateway accepts EVERY request (202, no 5xx,
//      no timeouts) — it only stores the file and enqueues a job, so it stays fast.
//   2. The spike piles up in the durable RabbitMQ queue instead of overloading
//      the workers.
//   3. After the spike, workers drain the queue and EVERY accepted video reaches
//      a terminal state. teardown() counts them: accepted == processed, lost == 0.
//
// Run (stack must be up — `make up`):
//   make spike                                   # default profile, live in Grafana
//   k6 run load/spike.js                         # plain, no Prometheus output
//   PEAK_RATE=40 SPIKE_DURATION=45s make spike   # bigger spike
//
// Tunables (env vars): BASE_URL, USERS, BASE_RATE, PEAK_RATE, SPIKE_DURATION,
// DRAIN_TIMEOUT (seconds), VIDEO (path to the upload fixture).

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';
import exec from 'k6/execution';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const USERS = parseInt(__ENV.USERS || '5', 10);
const BASE_RATE = parseInt(__ENV.BASE_RATE || '2', 10); // uploads/s before & after
const PEAK_RATE = parseInt(__ENV.PEAK_RATE || '25', 10); // uploads/s at the peak
const SPIKE_DURATION = __ENV.SPIKE_DURATION || '30s';
const DRAIN_TIMEOUT = parseInt(__ENV.DRAIN_TIMEOUT || '600', 10);
// 60s 720p clip (~230 KB): cheap to upload, ~0.5s of ffmpeg per job, so a
// spike visibly outpaces the workers and builds a backlog in the queue.
const VIDEO = __ENV.VIDEO || 'fixtures/spike.mp4';

// open() runs in the init context, once per VU; 'b' = binary.
const video = open(VIDEO, 'b');

// Custom counters, all exported to Prometheus as k6_<name>_total.
const uploadsAccepted = new Counter('uploads_accepted'); // 202s from the gateway
const uploadsRejected = new Counter('uploads_rejected'); // anything else
const videosDone = new Counter('videos_done'); // counted in teardown
const videosFailed = new Counter('videos_failed'); // processed, but FAILED
const videosLost = new Counter('videos_lost'); // accepted but never finished

export const options = {
  // Tag every metric so Grafana can isolate this run (make spike sets TESTID).
  tags: { testid: __ENV.TESTID || 'spike' },
  setupTimeout: '60s',
  teardownTimeout: `${DRAIN_TIMEOUT + 60}s`,
  scenarios: {
    spike: {
      // Arrival-rate: k6 starts N iterations per second no matter how slow the
      // server gets — a real traffic spike doesn't politely wait for responses.
      executor: 'ramping-arrival-rate',
      startRate: BASE_RATE,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 300,
      stages: [
        { target: BASE_RATE, duration: '20s' }, // normal traffic
        { target: PEAK_RATE, duration: '5s' }, // sudden spike
        { target: PEAK_RATE, duration: SPIKE_DURATION }, // hold the peak
        { target: BASE_RATE, duration: '5s' }, // back to normal
        { target: BASE_RATE, duration: '20s' },
      ],
    },
  },
  thresholds: {
    // The requirement, as pass/fail gates. Any breach turns the run red.
    'http_req_failed{name:upload}': ['rate==0'],
    uploads_rejected: ['count==0'],
    videos_lost: ['count==0'],
    // Accept stays fast even at the peak: work is queued, not done inline.
    'http_req_duration{name:upload}': ['p(95)<1500'],
  },
};

// setup() runs once: register fresh users for this run, so teardown can count
// exactly the videos this run created.
export function setup() {
  const run = Date.now();
  const tokens = [];
  for (let i = 0; i < USERS; i++) {
    const creds = JSON.stringify({ email: `spike-${run}-${i}@fiapx.local`, password: 'spike-pass-123' });
    const params = { headers: { 'Content-Type': 'application/json' } };
    const reg = http.post(`${BASE_URL}/auth/register`, creds, params);
    check(reg, { 'register 201': (r) => r.status === 201 });
    const login = http.post(`${BASE_URL}/auth/login`, creds, params);
    if (!check(login, { 'login 200': (r) => r.status === 200 })) {
      exec.test.abort(`login failed: ${login.status} ${login.body}`);
    }
    tokens.push(login.json('token'));
  }
  return { tokens };
}

// The load: each iteration is one video upload by one of the test users.
export default function (data) {
  const token = data.tokens[exec.scenario.iterationInTest % data.tokens.length];
  const res = http.post(
    `${BASE_URL}/videos`,
    { video: http.file(video, 'spike.mp4', 'video/mp4') },
    { headers: { Authorization: `Bearer ${token}` }, tags: { name: 'upload' }, timeout: '30s' },
  );
  const ok = check(res, { 'upload 202 Accepted': (r) => r.status === 202 });
  if (ok) {
    uploadsAccepted.add(1);
  } else {
    uploadsRejected.add(1);
  }
}

// teardown() runs once after the load: poll every test user's video list until
// nothing is PENDING/PROCESSING, then report what happened to each upload.
export function teardown(data) {
  const deadline = Date.now() + DRAIN_TIMEOUT * 1000;
  let counts;
  for (;;) {
    counts = { total: 0, done: 0, failed: 0, pending: 0 };
    for (const token of data.tokens) {
      const res = http.get(`${BASE_URL}/videos`, {
        headers: { Authorization: `Bearer ${token}` },
        tags: { name: 'drain-poll' },
      });
      for (const v of res.json('videos') || []) {
        counts.total++;
        if (v.status === 'DONE') counts.done++;
        else if (v.status === 'FAILED') counts.failed++;
        else counts.pending++;
      }
    }
    console.log(`drain: ${counts.done} done, ${counts.failed} failed, ${counts.pending} still queued/processing`);
    if (counts.pending === 0 || Date.now() > deadline) break;
    sleep(3);
  }

  videosDone.add(counts.done);
  videosFailed.add(counts.failed);
  // Stuck past the drain timeout counts as lost — the strict interpretation.
  videosLost.add(counts.pending);
  return counts;
}

// handleSummary prints a one-glance verdict under k6's normal summary.
export function handleSummary(data) {
  const n = (name) => (data.metrics[name] ? data.metrics[name].values.count : 0);
  const accepted = n('uploads_accepted');
  const rejected = n('uploads_rejected');
  const processed = n('videos_done') + n('videos_failed');
  // Accepted but never showed up as processed — lost jobs, or stuck ones.
  const lost = accepted - processed;
  const passed = rejected === 0 && lost === 0;

  const verdict = [
    '',
    '================ FIAP X — spike verdict ================',
    `  uploads sent      : ${accepted + rejected}`,
    `  accepted (202)    : ${accepted}`,
    `  rejected          : ${rejected}`,
    `  processed         : ${processed}  (done ${n('videos_done')}, failed ${n('videos_failed')})`,
    `  lost              : ${lost}`,
    `  RESULT            : ${passed ? 'PASS — no request lost during the spike' : 'FAIL'}`,
    '========================================================',
    '',
  ].join('\n');

  return {
    stdout: verdict,
    'results/spike-summary.json': JSON.stringify(data, null, 2),
  };
}
