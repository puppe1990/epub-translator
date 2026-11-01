(() => {
  const syncNavigation = () => {
    const page = location.pathname.startsWith("/settings") ? "settings" : "home";
    document.querySelectorAll("[data-studio-nav]").forEach((link) => {
      const active = link.getAttribute("data-studio-nav") === page;
      link.classList.toggle("is-current", active);
      if (active) link.setAttribute("aria-current", "page");
      else link.removeAttribute("aria-current");
    });
    const locationLabel = document.querySelector("[data-studio-location]");
    if (locationLabel)
      locationLabel.textContent = page === "settings" ? "Configurações" : "Biblioteca";
  };
  document.addEventListener("change", (event) => {
    if (event.target.id !== "epub-file") return;
    const file = event.target.files[0];
    const label = document.querySelector("[data-upload-filename]");
    const announcement = document.getElementById("file-selected");
    if (label) label.textContent = file ? file.name : "Escolher um livro";
    if (announcement)
      announcement.textContent = file
        ? `Arquivo selecionado: ${file.name}`
        : "Nenhum arquivo selecionado";
  });
  document.addEventListener("click", (event) => {
    if (event.target.closest("[data-chapter-preview-close]")) {
      event.preventDefault();
      const dialog = document.getElementById("chapter-preview-dialog");
      if (dialog?.open) dialog.close();
      return;
    }
    const link = event.target.closest("[data-chapter-preview]");
    if (!link) return;
    const dialog = document.getElementById("chapter-preview-dialog");
    const body = document.getElementById("chapter-preview-body");
    if (!dialog || !body || typeof dialog.showModal !== "function") return;
    event.preventDefault();
    body.innerHTML = '<p class="preview-empty">Carregando…</p>';
    dialog.showModal();
    fetch(link.href, {
      headers: { "X-Chapter-Preview": "dialog", Accept: "text/html" },
    })
      .then((response) => {
        if (!response.ok) throw new Error("preview HTTP " + response.status);
        return response.text();
      })
      .then((html) => {
        body.innerHTML = html;
      })
      .catch(() => {
        body.innerHTML = '<p class="preview-empty">Não consegui abrir este capítulo.</p>';
      });
  });
  document.addEventListener("amarra:morphed", syncNavigation);
  const themeButton = document.querySelector(".theme-toggle");
  const themeLabel = document.querySelector("[data-amarra-theme-label]");
  const lightTheme = document.documentElement.classList.contains("light");
  if (themeButton) themeButton.setAttribute("aria-pressed", String(lightTheme));
  if (themeLabel) themeLabel.textContent = lightTheme ? "Modo escuro" : "Modo claro";
  syncNavigation();
})();
