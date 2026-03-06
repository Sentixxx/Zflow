import { SettingsView } from "./SettingsView";
import type { SettingsViewProps } from "./SettingsView";

type SettingsModalProps = SettingsViewProps & {
  open: boolean;
  onClose: () => void;
};

export function SettingsModal({ open, onClose, ...viewProps }: SettingsModalProps) {
  if (!open) {
    return null;
  }

  return (
    <div className="modal-backdrop settings-backdrop" onClick={onClose}>
      <div className="settings-modal" onClick={(event) => event.stopPropagation()}>
        <div className="settings-modal-header">
          <h3>设置</h3>
          <button className="sidebar-toggle" onClick={onClose} aria-label="关闭设置">
            ✕
          </button>
        </div>
        <SettingsView {...viewProps} />
      </div>
    </div>
  );
}
