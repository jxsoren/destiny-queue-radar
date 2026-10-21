import type { AriaRole, ReactNode } from 'react';

export interface TooltipCost {
  label: string;
  have: number;
  need: number;
}

export interface TooltipProps {
  id?: string;
  role?: AriaRole;
  title: string;
  children: ReactNode;
  cost?: TooltipCost;
  className?: string;
}

/** Thorn's "Mark of the Devourer" tooltip: near-black title strip, charcoal body, green cost row. */
export function Tooltip({ id, role, title, children, cost, className = '' }: TooltipProps) {
  return (
    <div id={id} role={role} className={`tip ${className}`}>
      <p className="tip-title">{title}</p>
      <div className="tip-text">{children}</div>
      {cost && (
        <p className="tip-cost">
          <span>
            <span aria-hidden="true" className="tip-cube" />
            {cost.label}
          </span>
          <span>
            {cost.have} <span className="text-white">/ {cost.need}</span>
          </span>
        </p>
      )}
    </div>
  );
}
