/** Shared motion: one easing, one entrance, one stagger for the whole dashboard. */
export const EASE = [0.32, 0.72, 0, 1] as const;
export const STAGGER_S = 0.055;

/** Fade up with a small blur burn-off; `i` staggers siblings in reading order. */
export function enter(i = 0) {
  return {
    initial: { opacity: 0, y: 10, filter: "blur(4px)" },
    animate: { opacity: 1, y: 0, filter: "blur(0px)" },
    transition: { duration: 0.6, ease: EASE, delay: i * STAGGER_S },
  };
}
