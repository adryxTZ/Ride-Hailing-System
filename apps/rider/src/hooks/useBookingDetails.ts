/**
 * useBookingDetails — state for the details step: "Who is riding?" (me or
 * someone else), and for a package the sender (when the rider is not at the
 * pickup), the receiver and what is being sent. Seeded from the booking draft,
 * written back on continue.
 */
import { useState } from "react";
import type { ContactValue } from "@/components/booking/ContactCard";
import { useCurrentLocation } from "@/hooks/useCurrentLocation";
import { useService } from "@/hooks/useServices";
import { asksWhoRides, contactValid, pickupIsHere, receiverRequired } from "@/models/bookingDetails";
import { useBookingStore } from "@/services/bookingStore";

const MAX_NOTE = 120;

export function useBookingDetails() {
  const booking = useBookingStore();
  const { service } = useService(booking.serviceKey ?? "");
  const { point: here } = useCurrentLocation();
  const isPackage = receiverRequired(service);
  const askWho = asksWhoRides(service) && !pickupIsHere(booking.pickup, here);
  // undefined until the rider confirms who is at the pickup.
  const [forOther, setForOther] = useState<boolean | undefined>(booking.contacts.pickup ? true : undefined);
  const [atPickup, setAtPickup] = useState<ContactValue | undefined>(booking.contacts.pickup);
  const [receiver, setReceiver] = useState<ContactValue | undefined>(booking.contacts.dropoff);
  const [note, setNoteRaw] = useState(booking.notes ?? "");
  const setNote = (v: string) => setNoteRaw(v.slice(0, MAX_NOTE));

  const pickupOk = !askWho || forOther === false || (forOther === true && contactValid(atPickup));
  const receiverOk = !isPackage || contactValid(receiver);
  const complete = pickupOk && receiverOk;

  const clean = (c?: ContactValue) => (c ? { name: c.name.trim(), phone: c.phone.trim() } : undefined);
  /** Writes the answers into the booking draft. */
  const commit = () => {
    booking.setContacts({
      pickup: askWho && forOther ? clean(atPickup) : undefined,
      dropoff: isPackage ? clean(receiver) : undefined,
    });
    booking.setNotes(note.trim() || undefined);
  };

  return {
    service,
    isPackage,
    askWho,
    forOther,
    setForOther,
    atPickup,
    setAtPickup,
    receiver,
    setReceiver,
    note,
    setNote,
    complete,
    commit,
  };
}
