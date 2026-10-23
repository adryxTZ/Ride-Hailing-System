import { useLocalSearchParams } from "expo-router";
import { OtpScreen } from "@/screens/auth/OtpScreen";

export default function Otp() {
  const { phone, devCode } = useLocalSearchParams<{ phone: string; devCode?: string }>();
  return <OtpScreen phone={phone ?? ""} devCode={devCode} />;
}
