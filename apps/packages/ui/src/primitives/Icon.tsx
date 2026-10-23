import { MaterialCommunityIcons } from "@expo/vector-icons";
import type { ComponentProps } from "react";
import { useTheme } from "../ThemeProvider";

type Name = ComponentProps<typeof MaterialCommunityIcons>["name"];

/**
 * Icon — renders a MaterialCommunityIcons glyph by name. The catalogue stores
 * glyph names, so an operator can change a service's icon without a release.
 * Unknown names fall back to a neutral glyph instead of a blank.
 */
export function Icon({ name, size = 28, color }: { name?: string; size?: number; color?: string }) {
  const { palette } = useTheme();
  const glyph = (name && name in MaterialCommunityIcons.glyphMap ? name : "shape-outline") as Name;
  return <MaterialCommunityIcons name={glyph} size={size} color={color ?? palette.primary} />;
}
