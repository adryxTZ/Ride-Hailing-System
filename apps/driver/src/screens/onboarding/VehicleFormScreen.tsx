/** VehicleFormScreen — what you drive: the ride type (Bajaji, Standard, XL …), plate, make, model, colour, year. */
import { Button, Header, Icon, RadioRow, Screen, Section, Text, TextField, useTheme } from "@platform/ui";
import { useState } from "react";
import { StyleSheet, View } from "react-native";
import { SERVICE_KEY } from "@/config";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useBack } from "@/hooks/useBack";
import { useOnboarding, useSaveVehicle } from "@/hooks/useOnboarding";
import { localText, useTaxiClasses } from "@/hooks/useTaxiClasses";

const MIN_YEAR = 1950;
const MAX_YEAR = new Date().getFullYear() + 1;

export function VehicleFormScreen() {
  const back = useBack();
  const { palette } = useTheme();
  const { data: config } = useAppConfig();
  const locale = config?.brand.locale ?? "en";
  const { classes } = useTaxiClasses();
  const { data: ob } = useOnboarding();
  const save = useSaveVehicle();
  const v = ob?.vehicle;
  const [cls, setCls] = useState(v?.vehicle_class ?? "");
  const [plate, setPlate] = useState(v?.plate ?? "");
  const [make, setMake] = useState(v?.make ?? "");
  const [model, setModel] = useState(v?.model ?? "");
  const [colour, setColour] = useState(v?.colour ?? "");
  const [year, setYear] = useState(v?.year ? String(v.year) : "");
  const yearN = Number(year);
  const yearBad = year !== "" && (!Number.isInteger(yearN) || yearN < MIN_YEAR || yearN > MAX_YEAR);
  const ready = cls !== "" && plate.trim().length >= 4 && !yearBad;

  const submit = () =>
    save.mutate(
      {
        service: SERVICE_KEY,
        vehicle_class: cls,
        plate,
        make: make.trim() || undefined,
        model: model.trim() || undefined,
        colour: colour.trim() || undefined,
        year: year ? yearN : undefined,
      },
      { onSuccess: () => back() },
    );

  return (
    <Screen align="top">
      <Header title="Your vehicle" leading="back" onLeading={() => back()} />
      <Section title="Ride type">
        {classes.map((c) => (
          <View key={c.key} style={styles.row}>
            <Icon name={c.icon} size={24} color={palette.primary} />
            <View style={styles.flex}>
              <RadioRow
                label={localText(c.name, locale)}
                subtitle={localText(c.description, locale) || undefined}
                selected={cls === c.key}
                onPress={() => setCls(c.key)}
              />
            </View>
          </View>
        ))}
      </Section>
      <TextField
        label="Number plate"
        value={plate}
        onChangeText={setPlate}
        autoCapitalize="characters"
        placeholder="T 123 ABC"
      />
      <TextField label="Make" value={make} onChangeText={setMake} placeholder="Toyota" />
      <TextField label="Model" value={model} onChangeText={setModel} placeholder="IST" />
      <TextField label="Colour" value={colour} onChangeText={setColour} placeholder="Silver" />
      <TextField
        label="Year"
        value={year}
        onChangeText={setYear}
        keyboardType="number-pad"
        maxLength={4}
        error={yearBad ? `Between ${MIN_YEAR} and ${MAX_YEAR}` : undefined}
      />
      {v?.status === "APPROVED" ? (
        <Text variant="muted">Changing an approved vehicle sends it for review again.</Text>
      ) : null}
      {save.error ? <Text variant="error">{save.error.message}</Text> : null}
      <Button title="Save" onPress={submit} loading={save.isPending} disabled={!ready} />
    </Screen>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 12 },
  flex: { flex: 1 },
});
