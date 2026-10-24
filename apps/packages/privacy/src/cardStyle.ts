/** Card geometry shared by the privacy cards (design standard: 18px radius, soft shadow). */
import { StyleSheet } from "react-native";

/** Entrance timing for cards (eased, never a spring). */
export const ENTER_MS = 260;
export const ENTER_STAGGER_MS = 60;

export const cardStyles = StyleSheet.create({
  card: {
    borderRadius: 18,
    padding: 16,
    gap: 10,
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
});
