/**
 * DriverMap — the full-screen map behind the driver home. It follows the driver's
 * position and centres the camera in the area above the bottom sheet
 * (`bottomInset`), so the car marker and radar rings drawn at that centre sit on
 * the driver. Drawn by @platform/maps (MapLibre + OpenFreeMap, works in Expo Go).
 */
import { MapCanvas, type MapMarker } from "@platform/maps";
import { useTheme } from "@platform/ui";
import type { Point } from "@/services/presence";

const STREET_ZOOM = 15;
const SELF_FALLBACK = "#4285F4";

interface Props {
  centre: Point;
  /** Height covered by the bottom sheet; the camera centres above it. */
  bottomInset: number;
  /** Bump to re-centre on the driver after the user panned away. */
  recenterKey: number;
  /** On a trip: the car, the stop being driven to, and the line between them. */
  markers?: MapMarker[];
  route?: Point[];
  fitTo?: Point[];
}

export function DriverMap({ centre, bottomInset, recenterKey, markers, route, fitTo }: Props) {
  const { palette } = useTheme();
  return (
    <MapCanvas
      centre={centre}
      padding={{ top: 0, bottom: bottomInset }}
      initialZoom={STREET_ZOOM}
      recenterKey={recenterKey}
      markers={markers}
      route={route}
      fit={fitTo}
      colors={{
        primary: palette.primary,
        onPrimary: palette.on_primary ?? palette.background,
        text: palette.text,
        background: palette.background,
        self: palette.info ?? SELF_FALLBACK,
      }}
    />
  );
}
