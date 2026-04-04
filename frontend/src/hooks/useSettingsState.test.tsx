/* @vitest-environment jsdom */
import { describe, expect, it } from "vitest";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { useSettingsState } from "./useSettingsState";

describe("useSettingsState", () => {
  it("exposes defaults and updates settings", () => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);
    let snapshot: ReturnType<typeof useSettingsState> | null = null;

    const Probe = () => {
      snapshot = useSettingsState();
      return null;
    };

    act(() => {
      root.render(<Probe />);
    });

    expect(snapshot?.aiProtocol).toBe("openai");
    expect(snapshot?.scriptLang).toBe("shell");

    act(() => {
      snapshot?.setAIProtocol("anthropic");
    });

    expect(snapshot?.aiProtocol).toBe("anthropic");
  });
});
