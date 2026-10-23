"use client";
/** Card — the white rounded panel every section sits on (enters with the page). */
import { motion } from "motion/react";
import { enter } from "./motion";

interface Props {
  title?: string;
  action?: React.ReactNode;
  index?: number;
  className?: string;
  children: React.ReactNode;
}

export function Card({ title, action, index = 0, className = "", children }: Props) {
  return (
    <motion.section
      {...enter(index)}
      className={`min-w-0 rounded-xl border border-line bg-surface p-5 ${className}`}
      aria-label={title}
    >
      {title || action ? (
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          {title ? <h2 className="text-[15px] font-semibold">{title}</h2> : <span />}
          {action ? <div className="min-w-0 max-w-full">{action}</div> : null}
        </div>
      ) : null}
      {children}
    </motion.section>
  );
}
