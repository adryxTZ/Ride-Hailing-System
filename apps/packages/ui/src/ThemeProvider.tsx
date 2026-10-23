import { createContext, type PropsWithChildren, useContext } from "react";
import { bootstrapTheme, type Theme } from "./theme";

const ThemeContext = createContext<Theme>(bootstrapTheme);

/** Makes a theme available to the tree. Apps pass the server theme once loaded. */
export function ThemeProvider({ theme, children }: PropsWithChildren<{ theme?: Theme }>) {
  return <ThemeContext.Provider value={theme ?? bootstrapTheme}>{children}</ThemeContext.Provider>;
}

/** Active theme (palette, typography, assets). */
export function useTheme(): Theme {
  return useContext(ThemeContext);
}
