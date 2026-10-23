/**
 * RouteSummaryCard — "Route · Edit" with pickup, the stops (or an "Add stop" row
 * when there are none) and drop-off on a timeline (designs 4 and 6).
 */
import { Icon, Text, useTheme } from "@platform/ui";
import { Pressable, StyleSheet, View } from "react-native";

interface Props {
  pickup?: string;
  stops?: string[];
  dropoff?: string;
  onEdit?: () => void;
}

export function RouteSummaryCard({ pickup, stops = [], dropoff, onEdit }: Props) {
  const { palette } = useTheme();
  const middle = stops.length ? stops : [undefined];
  const rows: { key: string; text?: string; kind: "pickup" | "stop" | "add" | "dropoff" }[] = [
    { key: "pickup", text: pickup ?? "Pickup", kind: "pickup" },
    ...middle.map((s, i) => ({ key: `stop-${i}`, text: s, kind: (s ? "stop" : "add") as "stop" | "add" })),
    { key: "dropoff", text: dropoff ?? "Drop-off", kind: "dropoff" },
  ];
  return (
    <View style={[styles.card, { backgroundColor: palette.background }]}>
      <View style={styles.head}>
        <Text style={styles.title}>Route</Text>
        {onEdit ? (
          <Pressable onPress={onEdit} hitSlop={8}>
            <Text style={[styles.edit, { color: palette.primary }]}>Edit</Text>
          </Pressable>
        ) : null}
      </View>
      {rows.map((r, i) => (
        <View key={r.key} style={styles.row}>
          <View style={styles.rail}>
            <View style={[styles.stem, { backgroundColor: i === 0 ? "transparent" : palette.primary }]} />
            <View
              style={[
                r.kind === "pickup" || r.kind === "dropoff" ? styles.dot : styles.small,
                {
                  borderColor: r.kind === "dropoff" ? palette.text : palette.primary,
                  backgroundColor: r.kind === "dropoff" ? palette.text : palette.background,
                },
              ]}
            />
            <View
              style={[
                styles.stem,
                { backgroundColor: i === rows.length - 1 ? "transparent" : palette.primary },
              ]}
            />
          </View>
          {r.kind === "add" ? (
            <View style={styles.addStop}>
              <Text variant="muted" style={styles.text}>
                Add stop
              </Text>
              <Icon name="plus" size={24} color={palette.text} />
            </View>
          ) : (
            <Text style={[styles.text, styles.flex]} numberOfLines={1}>
              {r.text}
            </Text>
          )}
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  card: { borderRadius: 18, padding: 20, gap: 4 },
  head: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", marginBottom: 10 },
  title: { fontSize: 20, fontWeight: "700" },
  edit: { fontSize: 16, fontWeight: "700" },
  row: { flexDirection: "row", alignItems: "center", gap: 16, minHeight: 48 },
  rail: { width: 22, alignSelf: "stretch", alignItems: "center" },
  stem: { width: 2, flex: 1 },
  dot: { width: 20, height: 20, borderRadius: 10, borderWidth: 6 },
  small: { width: 12, height: 12, borderRadius: 6, borderWidth: 2 },
  text: { fontSize: 16 },
  flex: { flex: 1 },
  addStop: { flex: 1, flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
});
