import { useEffect, useRef, type KeyboardEvent, type ReactNode, type RefObject } from 'react';

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])';

export interface DialogProps {
  onClose: () => void;
  labelledBy: string;
  initialFocus?: RefObject<HTMLElement>;
  children: ReactNode;
}

/** A centred modal over the dimmed board. Mount it to open; unmount returns focus. */
export function Dialog({ onClose, labelledBy, initialFocus, children }: DialogProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const previous = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    (initialFocus?.current ?? ref.current)?.focus();
    return () => previous?.focus();
  }, [initialFocus]);

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key === 'Escape') {
      e.stopPropagation();
      onClose();
      return;
    }
    if (e.key !== 'Tab' || !ref.current) return;
    const items = [...ref.current.querySelectorAll<HTMLElement>(FOCUSABLE)];
    if (items.length === 0) {
      e.preventDefault();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  };

  return (
    <div
      data-testid="dialog-backdrop"
      className="fixed inset-0 z-40 overflow-y-auto flex items-start justify-center bg-scrim px-4 pt-[12vh]"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        tabIndex={-1}
        onKeyDown={onKeyDown}
        className="tip flex max-h-[calc(100dvh-12vh-1rem)] flex-col w-full max-w-[500px] border-t-0 shadow-[0_16px_50px_rgba(0,0,0,0.5)] outline-none"
      >
        {children}
      </div>
    </div>
  );
}
