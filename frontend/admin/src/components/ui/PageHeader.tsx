"use client";
/** PageHeader — big title, a muted line under it, and the page's main actions on the right. */
import { motion } from "motion/react";
import { enter } from "./motion";

export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
}) {
  return (
    <motion.div {...enter(0)} className="mb-5 flex flex-wrap items-end justify-between gap-4 pt-2">
      <div>
        <h1 className="text-[34px] font-semibold tracking-tight">{title}</h1>
        {subtitle ? <p className="mt-1 max-w-2xl text-sm text-muted">{subtitle}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </motion.div>
  );
}
