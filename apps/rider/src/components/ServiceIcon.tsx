/**
 * ServiceIcon — the picture on a service tile or option cell.
 *
 * Prefers a rendered image asset (assets/icons/services/<name>.png, 3D product-render
 * style like the taxi story reference) and falls back to the vector glyph tinted in the
 * brand primary when no asset exists for the catalogue icon name. Assets are generated
 * per brand with apps/tools/gen-icons.py and registered by apps/tools/sync-icons.sh.
 */
import { Icon, useTheme } from "@platform/ui";
import { Image } from "react-native";
import { serviceIcons } from "@/assets/serviceIcons";

export function ServiceIcon({ name, size, color }: { name?: string; size: number; color?: string }) {
  const { palette } = useTheme();
  const source = name ? serviceIcons[name] : undefined;
  if (source) {
    return <Image source={source} style={{ width: size, height: size }} resizeMode="contain" />;
  }
  return <Icon name={name} size={Math.round(size * 0.62)} color={color ?? palette.primary} />;
}
