export function splitTranslatedTextBlocks(raw: string | undefined): string[] {
  const text = (raw || "").replace(/\r\n/g, "\n").replace(/\r/g, "\n").trim();
  if (!text) {
    return [];
  }

  return text
    .split(/\n\s*\n+/)
    .map((block) => block.trim())
    .filter((block) => block.length > 0);
}

export type TranslationTemplate = {
  html: string;
  sources: string[];
};

const TRANSLATABLE_TAGS = new Set([
  "p",
  "li",
  "h1",
  "h2",
  "h3",
  "h4",
  "h5",
  "h6",
  "blockquote",
  "pre",
  "td",
  "th",
  "figcaption",
]);

export function buildTranslationTemplate(html: string | undefined): TranslationTemplate {
  const source = (html || "").trim();
  if (!source || typeof document === "undefined") {
    return { html: source, sources: [] };
  }

  const template = document.createElement("template");
  template.innerHTML = source;

  const sources: string[] = [];
  annotateTranslationTargets(template.content, sources);

  return {
    html: template.innerHTML,
    sources,
  };
}

export function renderTranslatedHTML(
  templateHTML: string | undefined,
  translationParagraphs: Array<{
    index: number;
    translated: string;
    status: "pending" | "done";
  }>,
  isTranslatingArticle: boolean,
): string {
  const source = (templateHTML || "").trim();
  if (!source || typeof document === "undefined") {
    return "";
  }

  const template = document.createElement("template");
  template.innerHTML = source;
  const translationByIndex = new Map(translationParagraphs.map((item) => [item.index, item]));

  const targets = Array.from(template.content.querySelectorAll<HTMLElement>("[data-translation-index]"));
  for (const target of targets) {
    const index = Number(target.dataset.translationIndex || "0");
    if (!Number.isFinite(index) || index <= 0) {
      continue;
    }

    const translation = translationByIndex.get(index);
    if (translation?.status === "done" && translation.translated.trim() !== "") {
      target.insertAdjacentElement("afterend", buildTranslationNode(translation.translated));
      continue;
    }

    if (isTranslatingArticle) {
      target.insertAdjacentElement("afterend", buildPendingNode());
    }
  }

  return template.innerHTML;
}

function annotateTranslationTargets(root: ParentNode, sources: string[]) {
  const childNodes = Array.from(root.childNodes);
  for (const child of childNodes) {
    if (child.nodeType === Node.TEXT_NODE) {
      const normalized = normalizeText(child.textContent || "");
      if (!normalized) {
        continue;
      }

      const paragraph = document.createElement("p");
      paragraph.textContent = normalized;
      paragraph.dataset.translationIndex = String(sources.length + 1);
      child.parentNode?.replaceChild(paragraph, child);
      sources.push(normalized);
      continue;
    }

    if (!(child instanceof Element)) {
      continue;
    }

    if (isTranslationTarget(child)) {
      child.setAttribute("data-translation-index", String(sources.length + 1));
      sources.push(normalizeText(child.textContent || ""));
      continue;
    }

    annotateTranslationTargets(child, sources);
  }
}

function isTranslationTarget(element: Element): boolean {
  const tag = element.tagName.toLowerCase();
  if (!TRANSLATABLE_TAGS.has(tag)) {
    return false;
  }

  const text = normalizeText(element.textContent || "");
  if (!text) {
    return false;
  }

  return !Array.from(element.children).some((child) => isTranslationTarget(child));
}

function buildTranslationNode(translated: string): HTMLDivElement {
  const wrapper = document.createElement("div");
  wrapper.className = "translation-block mt-3 space-y-3 border-l-2 border-primary/50 pl-4";

  for (const block of splitTranslatedTextBlocks(translated)) {
    const paragraph = document.createElement("div");
    paragraph.className = "text-base leading-[1.85] whitespace-pre-wrap break-words [overflow-wrap:anywhere]";
    paragraph.textContent = block;
    wrapper.appendChild(paragraph);
  }

  return wrapper;
}

function buildPendingNode(): HTMLDivElement {
  const wrapper = document.createElement("div");
  wrapper.className = "translation-pending mt-3 flex items-center gap-2 text-xs text-muted-foreground";
  wrapper.setAttribute("aria-live", "polite");

  const dot = document.createElement("span");
  dot.className = "w-1.5 h-1.5 rounded-full bg-primary animate-pulse";
  dot.setAttribute("aria-hidden", "true");

  const text = document.createElement("span");
  text.textContent = "该段翻译中...";

  wrapper.appendChild(dot);
  wrapper.appendChild(text);
  return wrapper;
}

function normalizeText(raw: string): string {
  return raw.replace(/\s+/g, " ").trim();
}
