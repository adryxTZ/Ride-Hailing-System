import { dayOptions, earliestPickup, pickupLabel, timeSlots } from "./schedule";

const now = new Date(2026, 9, 20, 12, 7); // Tue 20 Oct 2026 12:07

describe("schedule", () => {
  it("starts 30 minutes ahead on the 15-minute grid", () => {
    expect(earliestPickup(now).getHours()).toBe(12);
    expect(earliestPickup(now).getMinutes()).toBe(45);
  });
  it("offers seven days and today's remaining slots", () => {
    expect(
      dayOptions(now)
        .map((d) => d.label)
        .slice(0, 3),
    ).toEqual(["Today", "Tomorrow", "Thu 22"]);
    const today = timeSlots(dayOptions(now)[0].date, now);
    expect(today[0].getHours() * 60 + today[0].getMinutes()).toBe(12 * 60 + 45);
    expect(today.at(-1)?.getMinutes()).toBe(45);
  });
  it("labels pickups the way riders read them", () => {
    expect(pickupLabel(new Date(2026, 9, 21, 7, 15).toISOString(), now)).toBe("Tomorrow 07:15");
    expect(pickupLabel(new Date(2026, 9, 23, 9, 0).toISOString(), now)).toBe("Fri 23 Oct 09:00");
  });
});
