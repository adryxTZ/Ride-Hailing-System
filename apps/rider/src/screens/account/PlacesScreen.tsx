/** PlacesScreen — saved places: Home, Work and others (design: Saved places). Adding uses the device location for now; map picking lands with the request form. */
import { Button, Header, ListRow, Screen, Section, Text, TextField } from "@platform/ui";
import * as Location from "expo-location";
import { useState } from "react";
import { View } from "react-native";
import { useBack } from "@/hooks/useBack";
import { useDeletePlace, usePlaces, useSavePlace } from "@/hooks/useProfile";

type Kind = "HOME" | "WORK" | "OTHER";

export function PlacesScreen() {
  const back = useBack();
  const { data: places } = usePlaces();
  const save = useSavePlace();
  const remove = useDeletePlace();
  const [adding, setAdding] = useState<Kind | null>(null);
  const [label, setLabel] = useState("");
  const [address, setAddress] = useState("");
  const [error, setError] = useState<string>();

  const home = places?.find((p) => p.kind === "HOME");
  const work = places?.find((p) => p.kind === "WORK");
  const others = places?.filter((p) => p.kind === "OTHER") ?? [];

  const submit = async () => {
    setError(undefined);
    const perm = await Location.requestForegroundPermissionsAsync();
    if (perm.status !== "granted") {
      setError("Location permission is needed to save your current position.");
      return;
    }
    const pos = await Location.getCurrentPositionAsync({});
    save.mutate(
      {
        kind: adding as Kind,
        label: label.trim(),
        address: address.trim(),
        lat: pos.coords.latitude,
        lng: pos.coords.longitude,
      },
      {
        onSuccess: () => {
          setAdding(null);
          setLabel("");
          setAddress("");
        },
        onError: (e) => setError(e.message),
      },
    );
  };

  return (
    <Screen align="top">
      <Header title="Saved places" leading="back" onLeading={() => back()} />
      {adding ? (
        <View style={{ gap: 12 }}>
          <Text variant="muted">
            Saves your current location as {adding === "OTHER" ? "a place" : adding.toLowerCase()}.
          </Text>
          {adding === "OTHER" ? (
            <TextField label="Name" value={label} onChangeText={setLabel} placeholder="e.g. Gym" />
          ) : null}
          <TextField
            label="Address"
            value={address}
            onChangeText={setAddress}
            placeholder="Street, area"
            error={error}
          />
          <Button
            title="Save"
            onPress={() => void submit()}
            loading={save.isPending}
            disabled={address.trim().length < 3 || (adding === "OTHER" && label.trim().length < 2)}
          />
          <Button title="Cancel" variant="secondary" onPress={() => setAdding(null)} />
        </View>
      ) : (
        <Section>
          <ListRow
            icon="home-outline"
            title={home ? "Home" : "Add home address"}
            subtitle={home?.address}
            onPress={() => setAdding("HOME")}
          />
          <ListRow
            icon="briefcase-outline"
            title={work ? "Work" : "Add work address"}
            subtitle={work?.address}
            onPress={() => setAdding("WORK")}
          />
          {others.map((p) => (
            <ListRow
              key={p.id}
              icon="map-marker-outline"
              title={p.label}
              subtitle={p.address}
              onPress={() => remove.mutate(p.id)}
              chevron={false}
            />
          ))}
          <ListRow icon="plus" title="Add a place" onPress={() => setAdding("OTHER")} />
        </Section>
      )}
    </Screen>
  );
}
