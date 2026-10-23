/**
 * RouteFields — design 2's bordered card: pickup on top, any stops, then the
 * drop-off, joined by the timeline. The field being edited is a search input;
 * the others are tappable labels. "+" adds a stop (up to 3), swap flips
 * pickup and drop-off, the pin opens "choose on the map" for the edited field.
 */
import { Icon, Text, useTheme, withAlpha } from "@platform/ui";
import { Pressable, StyleSheet, TextInput, View } from "react-native";
import Animated, { FadeIn, LinearTransition } from "react-native-reanimated";

/** Which field is being edited: pickup, drop-off, or stop number n (0-based). */
export type Field = "pickup" | "dropoff" | number;

interface Props {
  pickupLabel: string;
  dropoffLabel?: string;
  stopLabels: (string | undefined)[];
  canAddStop: boolean;
  editing: Field;
  query: string;
  onQuery: (q: string) => void;
  onEdit: (f: Field) => void;
  onMap: () => void;
  onSwap: () => void;
  onAddStop: () => void;
  onRemoveStop: (index: number) => void;
}

export function RouteFields(p: Props) {
  const { palette } = useTheme();
  const rows: { field: Field; text?: string; placeholder: string }[] = [
    { field: "pickup", text: p.pickupLabel, placeholder: "Pickup location" },
    ...p.stopLabels.map((text, i) => ({ field: i as Field, text, placeholder: `Stop ${i + 1}` })),
    { field: "dropoff", text: p.dropoffLabel, placeholder: "Dropoff location" },
  ];
  return (
    <View style={styles.wrap}>
      <Animated.View
        layout={LinearTransition.duration(220)}
        style={[styles.card, { borderColor: palette.primary }]}
      >
        {rows.map((row, i) => (
          <View key={String(row.field)}>
            {i > 0 ? <View style={[styles.divider, { backgroundColor: palette.border }]} /> : null}
            <Animated.View entering={FadeIn.duration(200)} style={styles.row}>
              <Dot kind={row.field} first={i === 0} last={i === rows.length - 1} />
              {p.editing === row.field ? (
                <SearchInput placeholder={row.placeholder} query={p.query} onQuery={p.onQuery} />
              ) : (
                <Pressable onPress={() => p.onEdit(row.field)} style={styles.labelWrap}>
                  <Text
                    style={[styles.label, { color: row.text ? palette.text : palette.muted }]}
                    numberOfLines={1}
                  >
                    {row.text ?? row.placeholder}
                  </Text>
                </Pressable>
              )}
              {typeof row.field === "number" ? (
                <Pressable
                  accessibilityLabel={`Remove stop ${row.field + 1}`}
                  onPress={() => p.onRemoveStop(row.field as number)}
                  hitSlop={8}
                >
                  <Icon name="close" size={20} color={palette.muted} />
                </Pressable>
              ) : null}
              {p.editing === row.field ? (
                <Pressable
                  accessibilityLabel="Choose on map"
                  onPress={p.onMap}
                  style={[
                    styles.mapBtn,
                    { backgroundColor: withAlpha(palette.border ?? palette.surface, 0.9) },
                  ]}
                >
                  <Icon name="map-marker" size={22} color={palette.text} />
                </Pressable>
              ) : null}
            </Animated.View>
          </View>
        ))}
      </Animated.View>
      <View style={styles.side}>
        <Pressable accessibilityLabel="Add stop" onPress={p.onAddStop} disabled={!p.canAddStop} hitSlop={8}>
          <Icon name="plus" size={28} color={p.canAddStop ? palette.text : palette.border} />
        </Pressable>
        <Pressable accessibilityLabel="Swap pickup and drop-off" onPress={p.onSwap} hitSlop={8}>
          <Icon name="swap-vertical" size={28} color={palette.text} />
        </Pressable>
      </View>
    </View>
  );
}

function SearchInput({
  placeholder,
  query,
  onQuery,
}: {
  placeholder: string;
  query: string;
  onQuery: (q: string) => void;
}) {
  const { palette } = useTheme();
  return (
    <>
      <TextInput
        autoFocus
        value={query}
        onChangeText={onQuery}
        placeholder={placeholder}
        placeholderTextColor={palette.muted}
        style={[styles.input, { color: palette.text }]}
        returnKeyType="search"
        autoCorrect={false}
      />
      {query ? (
        <Pressable accessibilityLabel="Clear" onPress={() => onQuery("")} hitSlop={8}>
          <Icon name="close-circle" size={20} color={palette.muted} />
        </Pressable>
      ) : null}
    </>
  );
}

/** Timeline marker: brand ring for pickup, small ring for stops, dark ring for drop-off. */
function Dot({ kind, first, last }: { kind: Field; first: boolean; last: boolean }) {
  const { palette } = useTheme();
  const stop = typeof kind === "number";
  const colour = kind === "pickup" ? palette.primary : palette.text;
  return (
    <View style={styles.dotCol}>
      <View style={[styles.stem, { backgroundColor: first ? "transparent" : palette.text }]} />
      <View
        style={[
          stop ? styles.smallDot : styles.dot,
          { borderColor: colour, backgroundColor: palette.background },
        ]}
      />
      <View style={[styles.stem, { backgroundColor: last ? "transparent" : palette.text }]} />
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { flexDirection: "row", alignItems: "center", gap: 12 },
  card: { flex: 1, borderWidth: 2, borderRadius: 14, paddingLeft: 12, paddingRight: 8 },
  row: { flexDirection: "row", alignItems: "center", minHeight: 60, gap: 10 },
  dotCol: { width: 20, alignSelf: "stretch", alignItems: "center" },
  stem: { width: 2, flex: 1 },
  dot: { width: 16, height: 16, borderRadius: 8, borderWidth: 5 },
  smallDot: { width: 12, height: 12, borderRadius: 6, borderWidth: 3 },
  labelWrap: { flex: 1, justifyContent: "center", minHeight: 60 },
  input: { flex: 1, fontSize: 17 },
  label: { fontSize: 17 },
  divider: { height: 1, marginLeft: 30 },
  mapBtn: { width: 40, height: 40, borderRadius: 10, alignItems: "center", justifyContent: "center" },
  side: { gap: 28, alignItems: "center" },
});
