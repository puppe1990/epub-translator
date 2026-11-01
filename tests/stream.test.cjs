const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function shippedStream() {
  const bundle = fs.readFileSync(path.join(__dirname, "../web/static/js/amarra.js"), "utf8");
  // Exercise the actual shipped client without booting its browser shell.
  const exposed = bundle.replace(
    "  function boot() {",
    "  globalThis.testStream = stream_exports;\n  function boot() {"
  );
  const sandbox = {};
  vm.runInNewContext(exposed, sandbox);
  return sandbox.testStream;
}

class FakeEventSource {
  listeners = new Map();
  addEventListener(kind, listener) {
    this.listeners.set(kind, listener);
  }
  emit(kind, lastEventId, data) {
    this.listeners.get(kind)({ lastEventId, data });
  }
}

test("SSE updates each target named in lastEventId", () => {
  const targets = new Map(
    ["book-progress", "chapter-list", "book-actions"].map((id) => [id, { id }])
  );
  const updates = [];
  const source = shippedStream().connect("/books/1/events", {
    EventSource: FakeEventSource,
    document: { getElementById: (id) => targets.get(id) },
    morphFn: (element, html) => updates.push({ id: element.id, html }),
  });
  source.emit("morph", "book-progress", "<p>Pausado</p>");
  source.emit("morph", "book-actions", "<button>Retomar tradução</button>");
  assert.deepEqual(updates, [
    { id: "book-progress", html: "<p>Pausado</p>" },
    { id: "book-actions", html: "<button>Retomar tradução</button>" },
  ]);
});

test("SSE without an id still uses the configured default target", () => {
  let updated;
  const source = shippedStream().connect("/events", {
    EventSource: FakeEventSource,
    defaultTarget: "fallback",
    document: { getElementById: (id) => ({ id }) },
    morphFn: (element) => {
      updated = element.id;
    },
  });
  source.emit("morph", "", "<p>Atualização</p>");
  assert.equal(updated, "fallback");
});
