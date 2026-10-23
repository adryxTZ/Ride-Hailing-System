/**
 * MapCanvas (native) — MapLibre in a WebView. The page loads once; every prop
 * change is pushed as one state object. Until the page says "ready" the latest
 * state is kept and sent on ready, so nothing is lost during load.
 */
import { useEffect, useRef, useState } from "react";
import { StyleSheet } from "react-native";
import { WebView, type WebViewMessageEvent } from "react-native-webview";
import { DEFAULT_STYLE, mapHtml } from "./html";
import { DEFAULT_ZOOM, parseMessage, toState } from "./state";
import type { MapCanvasProps } from "./types";

export function MapCanvas(props: MapCanvasProps) {
  const ref = useRef<WebView>(null);
  const ready = useRef(false);
  const state = toState(props);
  const json = JSON.stringify(state);
  const latest = useRef(json);
  latest.current = json;
  // The page is built once from the first centre; later centres move the camera instead.
  const [html] = useState(() =>
    mapHtml(props.styleUrl ?? DEFAULT_STYLE, props.centre, props.initialZoom ?? DEFAULT_ZOOM),
  ); // built once; later centres move the camera

  useEffect(() => {
    if (ready.current) ref.current?.injectJavaScript(`window.__update(${json});true;`);
  }, [json]);

  const onMessage = (e: WebViewMessageEvent) => {
    const msg = parseMessage(e.nativeEvent.data);
    if (msg?.type === "ready") {
      ready.current = true;
      ref.current?.injectJavaScript(`window.__update(${latest.current});true;`);
    } else if (msg?.type === "centre") {
      props.onCentreChange?.({ lat: msg.lat, lng: msg.lng });
    }
  };

  return (
    <WebView
      ref={ref}
      style={StyleSheet.absoluteFill}
      originWhitelist={["*"]}
      source={{ html }}
      onMessage={onMessage}
      javaScriptEnabled
      scrollEnabled={false}
      overScrollMode="never"
      setBuiltInZoomControls={false}
      androidLayerType="hardware"
    />
  );
}
