const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { test } = require("node:test");

function loadWorkspace() {
  const listeners = new Map();
  const dialog = {
    open: false,
    showModal() {
      this.open = true;
    },
    close() {
      this.open = false;
    },
  };
  const body = { innerHTML: "" };
  const fetchCalls = [];
  const sandbox = {
    document: {
      addEventListener(type, fn) {
        listeners.set(type, fn);
      },
      querySelectorAll() {
        return [];
      },
      querySelector() {
        return null;
      },
      getElementById(id) {
        if (id === "chapter-preview-dialog") return dialog;
        if (id === "chapter-preview-body") return body;
        return null;
      },
      documentElement: { classList: { contains: () => true } },
    },
    location: { pathname: "/books/1" },
    fetch(url, opts) {
      fetchCalls.push({ url, opts });
      return Promise.resolve({
        ok: true,
        text: async () => '<article class="chapter-preview-spread">Hello one. Olá um.</article>',
      });
    },
  };
  vm.runInNewContext(
    fs.readFileSync(path.join(__dirname, "../web/static/js/workspace.js"), "utf8"),
    sandbox
  );
  return { listeners, dialog, body, fetchCalls };
}

test("Preview click opens the dialog with original and translated fragment", async () => {
  const { listeners, dialog, body, fetchCalls } = loadWorkspace();
  const link = {
    href: "/books/1/chapters/2/preview",
    closest(selector) {
      return selector === "[data-chapter-preview]" ? this : null;
    },
  };
  let prevented = false;
  listeners.get("click")({
    target: link,
    preventDefault() {
      prevented = true;
    },
  });

  assert.equal(prevented, true);
  assert.equal(dialog.open, true);
  assert.equal(fetchCalls.length, 1);
  assert.equal(fetchCalls[0].url, "/books/1/chapters/2/preview");
  assert.equal(fetchCalls[0].opts.headers["X-Chapter-Preview"], "dialog");

  await new Promise((resolve) => setImmediate(resolve));
  assert.match(body.innerHTML, /Hello one/);
  assert.match(body.innerHTML, /Olá um/);
});
