import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { SettingsView } from "./SettingsView";
import type { SettingsViewProps } from "./SettingsView";

type SettingsModalProps = SettingsViewProps & {
  open: boolean;
  onClose: () => void;
};

export function SettingsModal({ open, onClose, ...viewProps }: SettingsModalProps) {
  return (
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) onClose(); }}>
      <DialogContent className="max-w-2xl max-h-[85vh] flex flex-col gap-0 p-0 overflow-hidden">
        <DialogHeader className="px-6 py-4 border-b shrink-0">
          <DialogTitle>设置</DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-y-auto min-h-0">
          <SettingsView {...viewProps} />
        </div>
      </DialogContent>
    </Dialog>
  );
}
