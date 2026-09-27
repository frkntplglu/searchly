"use strict";

const defaultPerPage = 5;
const typeLabels = { video: "Video", article: "Makale" };
const sortLabels = { relevance: "alakalılığa göre", popularity: "popülerliğe göre" };

const state = {
  q: "",
  type: "",
  sort: "relevance",
  page: 1,
  perPage: defaultPerPage,
};

const el = {
  form: document.getElementById("search-form"),
  q: document.getElementById("q"),
  perPage: document.getElementById("per-page"),
  type: document.getElementById("type"),
  sort: document.getElementById("sort"),
  rows: document.getElementById("rows"),
  count: document.getElementById("count"),
  message: document.getElementById("message"),
  pager: document.getElementById("pager"),
  prev: document.getElementById("prev"),
  next: document.getElementById("next"),
  pageInfo: document.getElementById("page-info"),
  rowTemplate: document.getElementById("row-template"),
};

let controller;

async function load() {
  syncURL();

  // Cancel the previous request so a slow response cannot overwrite a newer one.
  controller?.abort();
  controller = new AbortController();

  const params = new URLSearchParams({ sort: state.sort, page: state.page, per_page: state.perPage });
  if (state.q) params.set("q", state.q);
  if (state.type) params.set("type", state.type);

  try {
    const res = await fetch(`/api/v1/contents?${params}`, { signal: controller.signal });
    const body = await res.json();
    if (!res.ok) throw new Error(body.error?.message || `İstek başarısız (${res.status})`);
    render(body);
  } catch (err) {
    if (err.name === "AbortError") return;
    renderError(err.message);
  }
}

function render({ data, pagination }) {
  el.rows.replaceChildren(...data.map(renderRow));
  // Relevance needs a keyword; without one the API sorts by popularity.
  const sort = state.q ? state.sort : "popularity";
  el.count.textContent = `${pagination.total} içerik · ${sortLabels[sort]} sıralı`;

  showMessage(data.length ? "" : state.q ? `"${state.q}" için sonuç bulunamadı.` : "Henüz içerik yok.");

  el.pager.hidden = pagination.total_pages <= 1;
  el.pageInfo.textContent = `${pagination.page} / ${Math.max(pagination.total_pages, 1)}`;
  el.prev.disabled = pagination.page <= 1;
  el.next.disabled = pagination.page >= pagination.total_pages;
}

// Rows are built with textContent only, so content from providers is never parsed as HTML.
function renderRow(content) {
  const row = el.rowTemplate.content.firstElementChild.cloneNode(true);
  row.querySelector(".title").textContent = content.title;
  row.querySelector(".meta").textContent =
    `${content.provider} · ${new Date(content.published_at).toLocaleDateString("tr-TR")}`;
  row.querySelector(".tags").replaceChildren(
    ...content.tags.map((t) => Object.assign(document.createElement("span"), { className: "tag", textContent: t })),
  );
  row.querySelector(".type").textContent = typeLabels[content.type] ?? content.type;
  row.querySelector(".score").textContent = content.score.toLocaleString("tr-TR", { maximumFractionDigits: 2 });
  return row;
}

function renderError(text) {
  el.rows.replaceChildren();
  el.count.textContent = "";
  el.pager.hidden = true;
  showMessage(text, true);
}

function showMessage(text, isError = false) {
  el.message.textContent = text;
  el.message.hidden = !text;
  el.message.classList.toggle("error", isError);
}

// Keeps the current search in the address bar so it can be shared or reloaded.
function syncURL() {
  const params = new URLSearchParams();
  if (state.q) params.set("q", state.q);
  if (state.type) params.set("type", state.type);
  if (state.sort !== "relevance") params.set("sort", state.sort);
  if (state.page > 1) params.set("page", state.page);
  if (state.perPage !== defaultPerPage) params.set("per_page", state.perPage);
  history.replaceState(null, "", params.size ? `?${params}` : location.pathname);
}

function readURL() {
  const params = new URLSearchParams(location.search);
  state.q = params.get("q") ?? "";
  state.type = params.get("type") ?? "";
  state.sort = params.get("sort") ?? "relevance";
  state.page = Number(params.get("page")) || 1;
  state.perPage = Number(params.get("per_page")) || defaultPerPage;

  el.q.value = state.q;
  state.perPage = Number(selectValue(el.perPage, String(state.perPage)));
  state.sort = selectValue(el.sort, state.sort);
  state.type = selectValue(el.type, state.type);
}

// selectValue selects value in a <select>, falling back to its first option
// when value is not offered (e.g. an edited URL), and returns what is actually
// selected so the request always matches what the user sees.
function selectValue(select, value) {
  select.value = value;
  if (select.selectedIndex === -1) select.selectedIndex = 0;
  return select.value;
}

function search() {
  state.page = 1;
  load();
}

let debounce;
el.q.addEventListener("input", () => {
  clearTimeout(debounce);
  debounce = setTimeout(() => {
    state.q = el.q.value.trim();
    search();
  }, 300);
});

el.form.addEventListener("submit", (e) => {
  e.preventDefault();
  clearTimeout(debounce);
  state.q = el.q.value.trim();
  search();
});

el.type.addEventListener("change", () => {
  state.type = el.type.value;
  search();
});

el.sort.addEventListener("change", () => {
  state.sort = el.sort.value;
  search();
});

el.perPage.addEventListener("change", () => {
  state.perPage = Number(el.perPage.value);
  search();
});

el.prev.addEventListener("click", () => {
  state.page--;
  load();
});

el.next.addEventListener("click", () => {
  state.page++;
  load();
});

readURL();
load();
