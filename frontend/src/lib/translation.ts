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
  "td",
  "th",
  "figcaption",
]);

// Tags that represent code/preformatted content — must never be translated.
const CODE_TAGS = new Set(["pre", "code", "kbd", "samp"]);

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
      insertTranslationAfter(target, buildTranslationNode(translation.translated));
      continue;
    }

    if (isTranslatingArticle) {
      insertTranslationAfter(target, buildPendingNode());
    }
  }

  return template.innerHTML;
}

// For table cells (td/th), append inside the cell to keep table structure valid.
// For grid/flex containers, wrap target + translation node together so the wrapper
// occupies the original grid/flex slot — inserting a bare sibling would shift all
// subsequent items by one slot, breaking grid-column/flex-order layouts.
// For all other elements, insert as a sibling after the element.
function insertTranslationAfter(target: HTMLElement, node: HTMLElement): void {
  const tag = target.tagName.toLowerCase();
  if (tag === "td" || tag === "th") {
    target.appendChild(node);
    return;
  }

  const parent = target.parentElement;
  if (parent && isGridOrFlexContainer(parent)) {
    // Wrap strategy: replace target with a wrapper that holds both nodes.
    // IMPORTANT: target's class/id/style/data-* are NOT moved — they stay on target.
    // The wrapper only receives layout-related attributes that affect grid/flex slot
    // placement, so the slot is preserved without breaking target's own selectors.
    const wrapper = document.createElement("div");
    copyLayoutAttributesToWrapper(target, wrapper);
    parent.replaceChild(wrapper, target);
    wrapper.appendChild(target);
    wrapper.appendChild(node);
    return;
  }

  target.insertAdjacentElement("afterend", node);
}

function annotateTranslationTargets(root: ParentNode, sources: string[]) {
  // Skip entire subtree if this root is a code-like element
  if (root instanceof Element && isCodeElement(root)) {
    return;
  }

  const childNodes = Array.from(root.childNodes);
  for (const child of childNodes) {
    if (child.nodeType === Node.TEXT_NODE) {
      const normalized = normalizeText(child.textContent || "");
      if (!normalized) {
        continue;
      }

      // Skip bare text nodes whose ancestor is a code-like element
      if (hasCodeAncestor(child)) {
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

    // Skip code-like elements and their entire subtree
    if (isCodeElement(child)) {
      continue;
    }

    if (isTranslationTarget(child)) {
      child.setAttribute("data-translation-index", String(sources.length + 1));
      // Use extractTranslatableText to exclude code-like subtrees from the source,
      // so inline <code>/<kbd>/<samp> content is not sent to the translation API.
      sources.push(extractTranslatableText(child));
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
  wrapper.className = "translation-block immersive-translation";

  for (const block of splitTranslatedTextBlocks(translated)) {
    const paragraph = document.createElement("div");
    paragraph.className = "whitespace-pre-wrap break-words [overflow-wrap:anywhere]";
    paragraph.textContent = block;
    wrapper.appendChild(paragraph);
  }

  return wrapper;
}

function buildPendingNode(): HTMLDivElement {
  const wrapper = document.createElement("div");
  wrapper.className = "translation-block immersive-translation-pending flex items-center gap-2 text-xs text-muted-foreground";
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

/**
 * Recursively collect text from an element, skipping CODE_TAGS subtrees entirely.
 * This ensures inline <code>/<kbd>/<samp>/<pre> content is excluded from the
 * translation source string — they must not be sent to the translation API.
 */
function extractTranslatableText(node: Node): string {
  if (node.nodeType === Node.TEXT_NODE) {
    return node.textContent || "";
  }
  if (node instanceof Element && isCodeElement(node)) {
    // Skip entire code-like subtree
    return "";
  }
  let result = "";
  for (const child of Array.from(node.childNodes)) {
    result += extractTranslatableText(child);
  }
  return normalizeText(result);
}

// Returns true if the element itself is a code/preformatted tag.
function isCodeElement(el: Element): boolean {
  return CODE_TAGS.has(el.tagName.toLowerCase());
}

// Walk up the DOM tree to check whether any ancestor is a code-like tag.
// Used to protect bare text nodes inside inline <code> from being wrapped.
function hasCodeAncestor(node: Node): boolean {
  let current: Node | null = node.parentNode;
  while (current !== null) {
    if (current instanceof Element && isCodeElement(current)) {
      return true;
    }
    current = current.parentNode;
  }
  return false;
}

// Tailwind class prefixes that control grid/flex slot placement.
// Only these classes are safe to copy to the wrapper — everything else stays on target.
const LAYOUT_CLASS_PREFIXES = [
  "col-span-",
  "row-span-",
  "col-start-",
  "col-end-",
  "row-start-",
  "row-end-",
  "order-",
  "self-",
  "justify-self-",
  "place-self-",
];

// Inline style properties that control grid/flex slot placement.
const LAYOUT_STYLE_PROPS: ReadonlyArray<string> = [
  "gridArea",
  "gridColumn",
  "gridRow",
  "order",
  "alignSelf",
  "justifySelf",
  "placeSelf",
];

/**
 * Copy only layout-relevant attributes from target to wrapper.
 * Target itself is NEVER mutated — its class/id/style/data-* all remain intact.
 * Wrapper only gets what is needed to occupy the correct grid/flex slot.
 */
function copyLayoutAttributesToWrapper(target: HTMLElement, wrapper: HTMLElement): void {
  // Extract Tailwind layout classes (whitelist only)
  const layoutClasses = Array.from(target.classList).filter((cls) =>
    LAYOUT_CLASS_PREFIXES.some((prefix) => cls.startsWith(prefix)),
  );
  if (layoutClasses.length > 0) {
    wrapper.className = layoutClasses.join(" ");
  }

  // Extract layout-relevant inline style properties
  const targetStyle = target.style;
  const wrapperStyleParts: string[] = [];
  for (const prop of LAYOUT_STYLE_PROPS) {
    const value = targetStyle.getPropertyValue(
      // Convert camelCase to kebab-case for getPropertyValue
      prop.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`),
    );
    if (value) {
      wrapperStyleParts.push(
        `${prop.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)}: ${value}`,
      );
    }
  }
  if (wrapperStyleParts.length > 0) {
    wrapper.setAttribute("style", wrapperStyleParts.join("; "));
  }
}

// Detect whether a container uses grid or flex layout by checking computed style.
// Falls back to false when window/getComputedStyle is unavailable (SSR / jsdom without CSS).
function isGridOrFlexContainer(el: HTMLElement): boolean {
  if (typeof window === "undefined" || typeof window.getComputedStyle !== "function") {
    return false;
  }
  try {
    const display = window.getComputedStyle(el).display;
    return display === "grid" || display === "inline-grid" || display === "flex" || display === "inline-flex";
  } catch {
    return false;
  }
}
