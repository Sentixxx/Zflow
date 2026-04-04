/* @vitest-environment jsdom */
import { describe, expect, it } from "vitest";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { useSettingsState } from "./useSettingsState";

type SettingsSnapshot = {
  aiProtocol: "openai" | "anthropic";
  scriptLang: "shell" | "python" | "javascript";
  setAIProtocol: (value: "openai" | "anthropic") => void;
};

describe("useSettingsState", () => {
  it("exposes defaults and updates settings", () => {
    const globalWithAct = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
    globalWithAct.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);
    const snapshotRef = { current: null as SettingsSnapshot | null };

    const Probe = () => {
      snapshotRef.current = useSettingsState() as SettingsSnapshot;
      return null;
    };

    act(() => {
      root.render(<Probe />);
    });

    const snapshot = snapshotRef.current as SettingsSnapshot;
    expect(snapshot.aiProtocol).toBe("openai");
    expect(snapshot.scriptLang).toBe("shell");

    act(() => {
      snapshotRef.current?.setAIProtocol("anthropic");
    });

    const updated = snapshotRef.current as SettingsSnapshot;
    expect(updated.aiProtocol).toBe("anthropic");
  });
});
