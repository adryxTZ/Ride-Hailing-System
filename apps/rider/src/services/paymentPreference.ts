/**
 * paymentPreference — the rider's preferred way to pay (cash or mobile money),
 * kept on the device so every booking starts with it. Secure store on phones,
 * localStorage on the web build (development only).
 */
import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

export type PaymentMethod = "CASH" | "MOBILE_MONEY";

const KEY = "platform.payment-method.v1";
const METHODS: PaymentMethod[] = ["CASH", "MOBILE_MONEY"];

export async function loadPaymentMethod(): Promise<PaymentMethod> {
  const raw =
    Platform.OS === "web" ? globalThis.localStorage?.getItem(KEY) : await SecureStore.getItemAsync(KEY);
  return METHODS.includes(raw as PaymentMethod) ? (raw as PaymentMethod) : "CASH";
}

export async function savePaymentMethod(method: PaymentMethod): Promise<void> {
  if (Platform.OS === "web") {
    globalThis.localStorage?.setItem(KEY, method);
    return;
  }
  await SecureStore.setItemAsync(KEY, method);
}
