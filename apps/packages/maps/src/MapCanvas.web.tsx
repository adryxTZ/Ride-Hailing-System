/** MapCanvas (web) — the same map page in an iframe, so browser tests show the real map. */
import { createElement, useEffect, useRef, useState } from "react";
import { DEFAULT_STYLE, mapHtml } from "./html";
import { DEFAULT_ZOOM, parseMessage, toState } from "./state";
import type { MapCanvasProps } from "./types";

type MapWindow = Window & { __update?: (s: unknown) => void };

export function MapCanvas(props: MapCanvasProps) {
  const frame = useRef<HTMLIFrameElement | null>(null);
  const ready = useRef(false);
  const state = toState(props);
  const latest = useRef(state);
  latest.current = state;
  const json = JSON.stringify(state);
  const [html] = useState(() =>
    mapHtml(props.styleUrl ?? DEFAULT_STYLE, props.centre, props.initialZoom ?? DEFAULT_ZOOM),
  ); // built once; later centres move the camera
  const push = () => (frame.current?.contentWindow as MapWindow | null)?.__update?.(latest.current);
  useEffect(() => {
    if (ready.current) push();
  }, [json]);

  const onCentre = props.onCentreChange;
  useEffect(() => {
    const handler = (e: MessageEvent) => {
      if (e.source !== frame.current?.contentWindow) return;
      const msg = parseMessage(e.data);
      if (msg?.type === "ready") {
        ready.current = true;
        push();
      } else if (msg?.type === "centre") onCentre?.({ lat: msg.lat, lng: msg.lng });
    };
    window.addEventListener("message", handler);
    return () => window.removeEventListener("message", handler);
  });

  return createElement("iframe", {
    ref: frame,
    srcDoc: html,
    title: "map",
    style: { position: "absolute", inset: 0, width: "100%", height: "100%", border: 0 },
  });
}
